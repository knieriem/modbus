package modtcp

import (
	"bytes"
	"errors"
	"io"
	"net"
	"time"

	"modbus"
	"modbus/rtu"
)

const (
	hdrSize     = 6 // Size of the MBAP header, without the Unit field
	mbapHdrSize = hdrSize + 1

	hdrPosTxnID   = 0
	hdrPosProtoID = 2
	hdrPosLen     = 4
	hdrPosUnit    = 6
)

var (
	ErrWrongProtocolID       = errors.New("tcp: wrong protocol ID")
	ErrTransactionIDMismatch = errors.New("tcp: mismatch of transaction ID")

	bo = modbus.ByteOrder
)

type Conn struct {
	conn          net.Conn
	buf           *bytes.Buffer
	transactionID uint16

	readMgr *rtu.ReadMgr
	ExitC   chan error

	OnReceiveError func(*Conn, error)
}

func NewNetConn(conn net.Conn) (m *Conn) {
	m = new(Conn)
	m.conn = conn

	m.buf = new(bytes.Buffer)

	var buf = make([]byte, 4096)
	rf := func() ([]byte, error) {
		n, err := conn.Read(buf)
		if err == nil {
			return buf[:n], nil
		}
		return nil, err
	}
	m.ExitC = make(chan error, 1)
	m.readMgr = rtu.NewReadMgr(rf, m.ExitC)
	m.readMgr.MsgComplete = func(buf []byte) (complete bool) {
		if len(buf) < hdrSize {
			return
		}
		length := bo.Uint16(buf[hdrPosLen:])
		if len(buf) >= int(length+hdrSize) {
			complete = true
		}
		return
	}
	return
}

func (m *Conn) SetIntrC(c <-chan error) {
	m.readMgr.IntrC = c
}

func (m *Conn) Name() string {
	return "tcp"
}

func (m *Conn) MsgWriter() (w io.Writer) {
	b := m.buf
	b.Reset()
	b.Write([]byte{0, 0, 0, 0, 0, 0})
	return b
}

func (m *Conn) Send() (buf []byte, err error) {
	b := m.buf
	err = m.readMgr.Start()
	if err != nil {
		return
	}
	buf = b.Bytes()
	m.transactionID++
	bo.PutUint16(buf[hdrPosTxnID:], m.transactionID)
	bo.PutUint16(buf[hdrPosLen:], uint16(len(buf[hdrSize:])))

	_, err = b.WriteTo(m.conn)
	if err != nil {
		m.readMgr.Cancel()
	}
	return
}

func (m *Conn) Receive(tMax time.Duration, verifyLen func(int) error) (buf, msg []byte, err error) {
	if f := m.OnReceiveError; f != nil {
		defer func() {
			if err != nil {
				f(m, err)
			}
		}()
	}
retry:
	buf, err = m.readMgr.Read(tMax, tMax)
	if err != nil {
		return
	}
	n := len(buf)
	if n < 8 {
		err = modbus.ErrMsgTooShort
		return
	}
	if int(bo.Uint16(buf[hdrPosProtoID:])) != 0 {
		err = ErrWrongProtocolID
		return
	}
	length := int(bo.Uint16(buf[hdrPosLen:])) + hdrSize
	switch {
	case n < length:
		err = modbus.ErrMsgTooShort
		return
	case n > length:
		err = modbus.ErrMsgTooLong
		return
	}
	err = verifyLen(n - 8)
	if err != nil {
		return
	}
	tID := bo.Uint16(buf[hdrPosTxnID:])
	switch {
	case tID < m.transactionID:
		err = m.readMgr.Start()
		if err != nil {
			return
		}
		goto retry
	case tID != m.transactionID:
		err = ErrTransactionIDMismatch
		return
	}
	msg = buf[hdrSize:]
	return
}
