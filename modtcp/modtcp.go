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

var (
	ErrWrongProtocolID       = errors.New("tcp: wrong protocol ID")
	ErrTransactionIDMismatch = errors.New("tcp: mismatch of transaction ID")
)

type Mode struct {
	conn          net.Conn
	buf           *bytes.Buffer
	transactionID uint16

	readMgr *rtu.ReadMgr
	ExitC   chan int

	OnReceiveError func(*Mode, error)
}

func NewTransmissionMode(conn net.Conn) (m *Mode) {
	m = new(Mode)
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
	m.ExitC = make(chan int, 1)
	m.readMgr = rtu.NewReadMgr(rf, m.ExitC)
	m.readMgr.MsgComplete = func(buf []byte) (complete bool) {
		if len(buf) < 6 {
			return
		}
		length := modbus.ByteOrder.Uint16(buf[4:])
		if len(buf) >= int(length+6) {
			complete = true
		}
		return
	}
	return
}

func (m *Mode) Name() string {
	return "tcp"
}

func (m *Mode) MsgWriter() (w io.Writer) {
	b := m.buf
	b.Reset()
	b.Write([]byte{0, 0, 0, 0, 0, 0})
	return b
}

func (m *Mode) Send() (buf []byte, err error) {
	b := m.buf
	err = m.readMgr.Start()
	if err != nil {
		return
	}
	buf = b.Bytes()
	m.transactionID++
	modbus.ByteOrder.PutUint16(buf, m.transactionID)
	modbus.ByteOrder.PutUint16(buf[4:], uint16(len(buf[6:])))

	_, err = b.WriteTo(m.conn)
	if err != nil {
		m.readMgr.Cancel()
	}
	return
}

func (m *Mode) Receive(tMax time.Duration, verifyLen func(int) error) (buf, msg []byte, err error) {
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
	if int(modbus.ByteOrder.Uint16(buf[2:])) != 0 {
		err = ErrWrongProtocolID
		return
	}
	length := int(modbus.ByteOrder.Uint16(buf[4:])) + 6
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
	tID := modbus.ByteOrder.Uint16(buf)
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
	msg = buf[6:]
	return
}
