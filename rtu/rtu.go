package rtu

import (
	"bytes"
	"io"
	"time"

	"modbus"
	"modbus/hash"
	"modbus/hash/crc16"
)

type Mode struct {
	conn io.ReadWriter
	buf  *bytes.Buffer

	readMgr *ReadMgr
	ExitC   chan int

	h hash.Hash16

	InterframeTimeout time.Duration
	OnReceiveError    func(*Mode, error)
}

func NewTransmissionMode(conn io.ReadWriter) (m *Mode) {
	m = new(Mode)
	m.conn = conn

	m.buf = new(bytes.Buffer)

	var buf = make([]byte, 64)
	rf := func() ([]byte, error) {
		n, err := conn.Read(buf)
		if err == nil {
			return buf[:n], nil
		}
		return nil, err
	}
	m.ExitC = make(chan int, 1)
	m.readMgr = NewReadMgr(rf, m.ExitC)

	m.InterframeTimeout = 5 * time.Millisecond
	return
}

var crcTab = crc16.MakeTable(crc16.IBMCRC)

func (m *Mode) Name() string {
	return "rtu"
}

func (m *Mode) MsgWriter() (w io.Writer) {
	b := m.buf
	b.Reset()
	m.h = crc16.New(crcTab)
	return io.MultiWriter(b, m.h)
}

func (m *Mode) Send() (buf []byte, err error) {
	b := m.buf
	crc := m.h.Sum16()
	b.WriteByte(byte(crc & 0xFF))
	b.WriteByte(byte(crc >> 8))

	m.readMgr.Start()
	buf = b.Bytes()
	_, err = b.WriteTo(m.conn)
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
	buf, err = m.readMgr.Read(tMax, m.InterframeTimeout)
	if err != nil {
		return
	}
	n := len(buf)
	if n < 4 {
		err = modbus.ErrMsgTooShort
		return
	}
	err = verifyLen(n - 4)
	if err != nil {
		return
	}
	if crc16.Checksum(buf, crcTab) != 0 {
		err = modbus.ErrCRC
		return
	}
	msg = buf[:n-2]
	return
}

// In case the inter-char/inter-frame timeout is too short,
// a message might get truncated – the remaining bytes
// will be discarded, even if they could have been received,
// if the timeout had been a bit longer. MaybeTruncatedMsg
// tells if the error suggests such a condition.
func MaybeTruncatedMsg(err error) bool {
	switch err {
	default:
		return false
	case modbus.ErrMsgTooShort:
	case modbus.ErrInvalidMsgLen:
	}
	return true
}
