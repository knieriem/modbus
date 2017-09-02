package rtu

import (
	"bytes"
	"io"
	"time"

	"modbus"
	"te/hash"
	"te/hash/crc16"
)

type Conn struct {
	conn io.ReadWriter
	buf  *bytes.Buffer

	readMgr *ReadMgr
	ExitC   chan int

	h Hash

	InterframeTimeout time.Duration
	OnReceiveError    func(*Conn, error)
}

func NewNetConn(conn io.ReadWriter) (m *Conn) {
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
	m.ExitC = make(chan int, 1)
	m.h = NewHash()
	m.readMgr = NewReadMgr(rf, m.ExitC)

	m.InterframeTimeout = 5 * time.Millisecond
	return
}

var crcTab = crc16.MakeTable(crc16.IBMCRC)

type Hash struct {
	hash.Hash16
}

func NewHash() Hash {
	return Hash{Hash16: crc16.New(crcTab)}
}

func (h *Hash) Sum(in []byte) []byte {
	s := h.Sum16()
	return append(in, byte(s&0xFF), byte(s>>8))
}

func (m *Conn) SetIntrC(c <-chan error) {
	m.readMgr.IntrC = c
}

func (m *Conn) Name() string {
	return "rtu"
}

func (m *Conn) MsgWriter() (w io.Writer) {
	b := m.buf
	b.Reset()
	m.h.Reset()
	return io.MultiWriter(b, m.h)
}

func (m *Conn) Send() (buf []byte, err error) {
	b := m.buf
	b.Write(m.h.Sum(nil))

	err = m.readMgr.Start()
	if err != nil {
		return
	}
	buf = b.Bytes()
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

func (m *Conn) ReadMgr() *ReadMgr {
	return m.readMgr
}
