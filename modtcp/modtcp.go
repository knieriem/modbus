package modtcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/knieriem/modbus"
	"github.com/knieriem/serframe"
)

const (
	hdrSize     = 6 // Size of the MBAP header, without the Unit field
	mbapHdrSize = hdrSize + 1
	pduSize     = 256 - 2 - 1
	aduSizeMax  = mbapHdrSize + pduSize

	hdrPosTxnID   = 0
	hdrPosProtoID = 2
	hdrPosLen     = 4
	hdrPosUnit    = 6
	hdrPosPDU     = hdrPosUnit + 1
)

var (
	ErrWrongProtocolID       = errors.New("tcp: wrong protocol ID")
	ErrTransactionIDMismatch = errors.New("tcp: mismatch of transaction ID")

	bo = modbus.ByteOrder
)

type Conn struct {
	conn net.Conn
	buf  struct {
		w *bytes.Buffer
		r []byte
	}
	transactionID uint16

	readMgr *serframe.Stream
	ExitC   <-chan error

	OnReceiveError func(*Conn, error)
}

func NewNetConn(conn net.Conn) (m *Conn) {
	m = new(Conn)
	m.conn = conn

	m.buf.w = new(bytes.Buffer)
	m.buf.r = make([]byte, aduSizeMax)

	m.readMgr = serframe.NewStream(conn,
		serframe.WithReceptionOptions(
			serframe.WithFrameInterceptor(func(buf, newPart []byte) (serframe.FrameStatus, error) {
				if len(buf) < hdrSize {
					return serframe.None, nil
				}
				length := bo.Uint16(buf[hdrPosLen:])
				if len(buf) >= int(length+hdrSize) {
					return serframe.CompleteSkipTimeout, nil
				}
				return serframe.None, nil
			}),
		),
	)
	m.ExitC = m.readMgr.ExitC
	return
}

func (m *Conn) Name() string {
	return "tcp"
}

func (m *Conn) Device() interface{} {
	return m.conn
}

func (m *Conn) MsgWriter() (w io.Writer) {
	b := m.buf.w
	b.Reset()
	b.Write([]byte{0, 0, 0, 0, 0, 0})
	return b
}

func (m *Conn) Send() (adu modbus.ADU, err error) {
	b := m.buf.w
	buf := b.Bytes()
	m.transactionID++
	bo.PutUint16(buf[hdrPosTxnID:], m.transactionID)
	bo.PutUint16(buf[hdrPosLen:], uint16(len(buf[hdrSize:])))

	adu.PDUStart = mbapHdrSize
	adu.Bytes = buf
	err = m.readMgr.StartReception(m.buf.r)
	if err != nil {
		return adu, err
	}
	_, err = b.WriteTo(m.conn)
	if err != nil {
		m.readMgr.CancelReception()
	}
	return adu, err
}

func (m *Conn) Receive(ctx context.Context, tMax time.Duration, ls *modbus.ExpectedRespLenSpec) (adu modbus.ADU, err error) {
	if f := m.OnReceiveError; f != nil {
		defer func() {
			if err != nil {
				f(m, err)
			}
		}()
	}

retry:
	adu.PDUStart = mbapHdrSize
	adu.Bytes, err = m.readMgr.ReadFrame(ctx,
		serframe.WithInitialTimeout(tMax),
		serframe.WithInterByteTimeout(tMax),
	)
	if err != nil {
		return
	}
	buf := adu.Bytes
	n := len(buf)
	if n < mbapHdrSize+1 {
		err = modbus.NewInvalidLen(modbus.MsgContextADU, n, mbapHdrSize+1)
		return
	}
	if int(bo.Uint16(buf[hdrPosProtoID:])) != 0 {
		err = ErrWrongProtocolID
		return
	}
	length := int(bo.Uint16(buf[hdrPosLen:])) + hdrSize
	if n != length {
		err = modbus.NewInvalidLen(modbus.MsgContextADU, n, length)
		return
	}
	err = ls.CheckLen(buf[mbapHdrSize:])
	if err != nil {
		return
	}
	tID := bo.Uint16(buf[hdrPosTxnID:])
	switch {
	case tID < m.transactionID:
		err = m.readMgr.StartReception(m.buf.r)
		if err != nil {
			return
		}
		goto retry
	case tID != m.transactionID:
		err = ErrTransactionIDMismatch
		return
	}
	return
}
