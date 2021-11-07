package rtu

import (
	"bytes"
	"context"
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

	expectedLenSpec *modbus.ExpectedRespLenSpec
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
		if len(msg) < 4 {
			return false
		}
		if m.expectedLenSpec.CheckLen(msg[1:len(msg)-2]) != nil {
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

func (m *Conn) Name() string {
	return "rtu"
}

func (m *Conn) Device() interface{} {
	return m.conn
}

func (m *Conn) MsgWriter() (w io.Writer) {
	b := m.buf
	b.Reset()
	m.h.Reset()
	return io.MultiWriter(b, m.h)
}

var localEchoSetByEnv = os.Getenv("MODBUS_RTU_LOCAL_ECHO") == "1"

func (m *Conn) Send() (adu modbus.ADU, err error) {
	b := m.buf
	b.Write(m.h.Sum(nil))

	adu.PDUStart = 1
	adu.PDUEnd = -2
	adu.Bytes = b.Bytes()
	err = m.readMgr.Start()
	if err != nil {
		return adu, err
	}
	_, err = b.WriteTo(m.conn)
	if err != nil {
		m.readMgr.Cancel()
	}
	if m.LocalEcho || localEchoSetByEnv {
		m.readMgr.echo = adu.Bytes
	}
	if port, ok := m.conn.(serport.Port); ok {
		err = port.Drain()
	}
	return adu, err
}

func (m *Conn) EnableReceive() error {
	return m.readMgr.Start()
}

func (m *Conn) Receive(ctx context.Context, tMax time.Duration, ls *modbus.ExpectedRespLenSpec) (adu modbus.ADU, err error) {
	if f := m.OnReceiveError; f != nil {
		defer func() {
			if err != nil {
				f(m, err)
			}
		}()
	}
	m.h.Reset()
	m.expectedLenSpec = ls
	adu.Bytes, err = m.readMgr.Read(ctx, tMax, m.InterframeTimeout)
	adu.PDUStart = 1
	adu.PDUEnd = -2
	if err != nil {
		return
	}
	n := len(adu.Bytes)
	if n < 4 {
		err = modbus.NewInvalidLen(modbus.MsgContextADU, n, 4)
		return
	}
	err = ls.CheckLen(adu.Bytes[1 : n-2])
	if err != nil {
		return
	}
	if m.h.Sum16() != 0 {
		err = modbus.ErrCRC
		return
	}
	return
}

// In case the inter-char/inter-frame timeout is too short,
// a message might get truncated – the remaining bytes
// will be discarded, even if they could have been received,
// if the timeout had been a bit longer. MaybeTruncatedMsg
// tells if the error suggests such a condition.
func MaybeTruncatedMsg(err error) bool {
	e, ok := err.(modbus.InvalidLenError)
	if !ok {
		return false
	}
	return !e.TooLong()
}

func (m *Conn) ReadMgr() *ReadMgr {
	return m.readMgr
}
