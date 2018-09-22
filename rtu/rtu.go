package rtu

import (
	"bytes"
	"io"
	"os"
	"time"

	"github.com/knieriem/hash"
	"github.com/knieriem/hash/crc16"
	"github.com/knieriem/modbus"
	"github.com/knieriem/serport"
)

type Conn struct {
	conn io.ReadWriter
	buf  *bytes.Buffer

	readMgr *ReadMgr
	ExitC   chan error

	h Hash

	LocalEcho         bool
	InterframeTimeout time.Duration
	OnReceiveError    func(*Conn, error)

	verifyLen func(int) error
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
	m.ExitC = make(chan error, 1)
	m.h = NewHash()
	m.readMgr = NewReadMgr(rf, m.ExitC)
	m.readMgr.CheckBytes = func(bnew []byte, msg []byte) bool {
		m.h.Write(bnew)
		if m.h.Sum16() != 0 {
			return false
		}
		if m.verifyLen(len(msg)-4) != nil {
			return false
		}
		return true
	}

	m.InterframeTimeout = 50 * time.Millisecond
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

var localEchoSetByEnv = os.Getenv("MODBUS_RTU_LOCAL_ECHO") == "1"

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
	if m.LocalEcho || localEchoSetByEnv {
		m.readMgr.echo = buf
	}
	if port, ok := m.conn.(serport.Port); ok {
		err = port.Drain()
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
	m.h.Reset()
	m.verifyLen = verifyLen
	buf, err = m.readMgr.Read(tMax, m.InterframeTimeout)
	if err != nil {
		return
	}
	n := len(buf)
	if n < 4 {
		err = modbus.NewInvalidMsgLen(n, 4)
		return
	}
	err = verifyLen(n - 4)
	if err != nil {
		return
	}
	if m.h.Sum16() != 0 {
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
	e, ok := err.(modbus.InvalidMsgLenError)
	if !ok {
		return false
	}
	return !e.TooLong()
}

func (m *Conn) ReadMgr() *ReadMgr {
	return m.readMgr
}
