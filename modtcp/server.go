// Copyright 2012 The modbus Authors.  All rights reserved.
// For small parts have been copied from net/http/server.go:
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modtcp

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/knieriem/modbus"
)

// A Server defines parameters for running a Modbus/TCP server. A value
// for Server with only the Bus field configured is a valid configuration.
type Server struct {
	Addr               string        // TCP address to listen on, ":502" if empty
	Bus                modbus.Bus    // Requests are forwarded to this bus
	ReadTimeout        time.Duration // maximum duration before timing out read of the request
	WriteTimeout       time.Duration // maximum duration before timing out write of the response
	OnTimeout, OnError struct {
		SendException bool
	}

	// ConnState specifies an optional callback function that is
	// called when a client connection changes state. See the
	// ConnState type and associated constants for details.
	ConnState func(net.Conn, ConnState)
}

// A ConnState represents the state of a client connection to a server.
// It's used by the optional Server.ConnState hook.
type ConnState int

func (c ConnState) String() string {
	return http.ConnState(c).String()
}

const (
	// ConnState values, see net/http.ConnState
	StateNew    = ConnState(http.StateNew)
	StateActive = ConnState(http.StateActive)
	StateIdle   = ConnState(http.StateIdle)
	StateClosed = ConnState(http.StateClosed)
)

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections. If srv.Addr is
// blank, ":502" is used.
func (srv *Server) ListenAndServe() error {
	addr := srv.Addr
	if addr == "" {
		addr = ":502"
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return srv.Serve(l)
}

// Serve accepts incoming connections on the Listener l. Only one client
// is handled at a time.
func (srv *Server) Serve(l net.Listener) error {
	defer l.Close()
	for {
		origConn, err := l.Accept()
		if err != nil {
			return err
		}
		c := &conn{
			Conn:   origConn,
			rb:     bufio.NewReader(origConn),
			server: srv,
		}
		c.setState(StateNew)
		srv.handleConn(c)
		c.setState(StateClosed)
		c.Close()
	}
}

type conn struct {
	net.Conn
	rb     *bufio.Reader
	server *Server
}

func (c *conn) readFull(b []byte) error {
	if d := c.server.ReadTimeout; d != 0 {
		c.SetReadDeadline(time.Now().Add(d))
	}
	_, err := io.ReadFull(c.rb, b)
	return err
}

func (c *conn) setState(state ConnState) {
	if hook := c.server.ConnState; hook != nil {
		hook(c.Conn, ConnState(state))
	}
}

func (srv *Server) handleConn(c *conn) error {
	var hdr = make([]byte, mbapHdrSize)
	var msg = make([]byte, 256-2-1)
	var resp = make(rawMsg, hdrSize+256-2)

	for {
		c.setState(StateIdle)
		err := c.readFull(hdr)
		if err != nil {
			return err
		}
		c.setState(StateActive)
		txnID := bo.Uint16(hdr[hdrPosTxnID:])
		protoID := bo.Uint16(hdr[hdrPosProtoID:])
		length := int(bo.Uint16(hdr[hdrPosLen:]))
		switch length {
		case 0:
			return errors.New("modtcp: length field must not be zero")
		case 1:
			continue
		}

		unit := hdr[hdrPosUnit]
		length--
		if cap(msg) < length {
			msg = make([]byte, 2*length)
		}
		msg = msg[:length]
		err = c.readFull(msg)
		if err != nil {
			return err
		}
		if protoID != 0 {
			continue
		}

		fn := msg[0]
		resp := resp[:hdrSize]
		err = srv.Bus.Request(unit, fn, rawMsg(msg[1:]), &resp)
		if err != nil {
			switch e := err.(type) {
			case modbus.Exception:
				resp = append(resp, unit, 0x80|fn, byte(e))
			case modbus.Error:
				h := &srv.OnError
				if err == modbus.ErrTimeout {
					h = &srv.OnTimeout
				}
				if !h.SendException {
					continue
				}
				resp = append(resp, unit, 0x80|fn, byte(modbus.XGwTargetFailedToRespond))
			default:
				return err
			}
		}
		bo.PutUint16(resp[hdrPosLen:], uint16(len(resp[hdrSize:])))
		bo.PutUint16(resp[hdrPosTxnID:], txnID)
		if d := c.server.WriteTimeout; d != 0 {
			c.SetWriteDeadline(time.Now().Add(d))
		}
		_, err = c.Write(resp)
		if err != nil {
			return err
		}
	}
}

type rawMsg []byte

func (b *rawMsg) Decode(buf []byte) (err error) {
	n := hdrSize + len(buf)
	if cap(*b) < n {
		*b = make([]byte, 2*n)
	}
	*b = (*b)[:n]
	copy((*b)[hdrSize:], buf)
	return
}

func (b rawMsg) Encode(w io.Writer) (err error) {
	_, err = w.Write(b)
	return
}
