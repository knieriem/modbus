package rtu

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/knieriem/modbus"
)

type ReadMgr struct {
	buf         []byte
	echo        []byte
	req         chan []byte
	done        chan readResult
	errC        chan error
	eof         bool
	Forward     io.Writer
	MsgComplete func([]byte) bool
	CheckBytes  func(bnew []byte, msg []byte) bool
}

type ReadFunc func() ([]byte, error)

func NewReadMgr(rf ReadFunc, exitC chan<- error) *ReadMgr {
	m := new(ReadMgr)
	m.buf = make([]byte, 0, 256)
	m.req = make(chan []byte)
	m.done = make(chan readResult)
	m.errC = make(chan error)
	go m.handle(rf, exitC)
	return m
}

func (m *ReadMgr) Start() (err error) {
	if m.eof {
		err = io.EOF
		return
	}
	select {
	case err = <-m.errC:
		m.eof = true
	default:
		m.buf = m.buf[:0]
		m.req <- m.buf
	}
	return
}

func (m *ReadMgr) Cancel() {
	m.req <- nil
}

func (m *ReadMgr) Read(ctx context.Context, tMax, interframeTimeoutMax time.Duration) (buf []byte, err error) {
	if m.eof {
		err = io.EOF
		return
	}
	bufok := false
	if m.CheckBytes == nil {
		bufok = true
	}
	nto := 0
	interframeTimeout := 1750 * time.Microsecond
	ntoMax := int((interframeTimeoutMax + interframeTimeout - 1) / interframeTimeout)
	timeout := time.NewTimer(tMax)
	nSkip := 0
readLoop:
	for {
		select {
		case r := <-m.done:
			nb := len(m.buf)
			m.buf = r.data
			if m.CheckBytes != nil && m.echo == nil {
				bufok = m.CheckBytes(m.buf[nb:], m.buf[nSkip:])
			}
			if !timeout.Stop() {
				<-timeout.C
			}
			if r.err != nil {
				err = r.err
				close(m.req)
				m.eof = true
				return
			}
		reeval:
			if m.echo != nil {
				nEcho := len(m.echo)
				if len(m.buf) >= nEcho {
					tail := m.buf[nEcho:]
					if m.CheckBytes != nil && len(tail) != 0 {
						bufok = m.CheckBytes(tail, m.buf[nSkip:])
					}
					if !bytes.Equal(m.buf[:nEcho], m.echo) {
						err = modbus.ErrEchoMismatch
						break readLoop
					}
					nSkip = nEcho
					m.echo = nil
					if len(tail) != 0 {
						goto reeval
					}
				}
				timeout.Reset(tMax)
				break
			} else if m.MsgComplete != nil {
				if m.MsgComplete(m.buf[nSkip:]) {
					break readLoop
				}
			}
			if interframeTimeoutMax == 0 {
				break readLoop
			}
			nto = 0
			timeout.Reset(interframeTimeout)

		case <-timeout.C:
			if m.echo != nil {
				if len(m.buf[nSkip:]) != 0 {
					err = modbus.ErrInvalidEchoLen
				} else {
					err = modbus.ErrTimeout
				}
			} else if len(m.buf[nSkip:]) != 0 && !bufok && nto < ntoMax {
				nto++
				timeout.Reset(interframeTimeout)
				continue
			}
			break readLoop
		case <-ctx.Done():
			err = ctx.Err()
			break readLoop
		}
	}
	m.req <- nil

	buf = m.buf[nSkip:]
	if err == nil && len(buf) == 0 {
		err = modbus.ErrTimeout
	}
	return
}

type readResult struct {
	data []byte
	err  error
}

func (m *ReadMgr) handle(read ReadFunc, exitC chan<- error) {
	var termErr error
	var dest []byte
	var errC chan<- error

	data := make(chan readResult)
	go func() {
		var err error
		for {
			buf, err1 := read()
			data <- readResult{buf, err1}
			<-data
			if err1 != nil {
				err = err1
				break
			}
		}
		close(data)
		exitC <- err
	}()

loop:
	for {
		select {
		case errC <- termErr:
			close(errC)
			break loop
		case dest = <-m.req:
		case r, dataOk := <-data:
			if dest != nil {
				if r.err == nil {
					dest = append(dest, r.data...)
				}
			} else if m.Forward != nil {
				m.Forward.Write(r.data)
			}
			if dataOk {
				data <- readResult{}
			} else {
				data = nil
			}
			if dest != nil {
				select {
				case m.done <- readResult{dest, r.err}:
					if r.err != nil {
						break loop
					}
				case b := <-m.req:
					if m.Forward != nil {
						m.Forward.Write(dest)
					}
					dest = b
				}
			}
			if r.err != nil && errC == nil {
				termErr = r.err
				errC = m.errC
			}
		}
	}
	close(m.done)
}
