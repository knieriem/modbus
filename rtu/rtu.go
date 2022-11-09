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
	"github.com/knieriem/serframe"
	"github.com/knieriem/serport"
)

type Conn struct {
	conn io.ReadWriter
	buf  struct {
		w *bytes.Buffer
		r []byte
	}

	readMgr *serframe.Stream
	ExitC   <-chan error

	h Hash

	LocalEcho         bool
	InterframeTimeout time.Duration
	OnReceiveError    func(*Conn, error)

	expectedLenSpec *modbus.ExpectedRespLenSpec
}

func NewNetConn(conn io.ReadWriter) (m *Conn) {
	m = new(Conn)
	m.conn = conn

	m.buf.w = new(bytes.Buffer)
	// For the read buffer, reserve space for an optional echoed request frame,
	// and the response frame
	m.buf.r = make([]byte, 2*256)

	m.ExitC = make(chan error, 1)
	m.h = NewHash()

	m.readMgr = serframe.NewStream(conn,
		serframe.WithInternalBufSize(512),
		serframe.WithReceptionOptions(
			serframe.WithInterByteTimeout(1750*time.Microsecond),
			serframe.WithFrameInterceptor(func(msg, bnew []byte) (serframe.FrameStatus, error) {
				m.h.Write(bnew)
				if m.h.Sum16() != 0 {
					return serframe.None, nil
				}
				if len(msg) < 4 {
					return serframe.None, nil
				}
				if m.expectedLenSpec.CheckLen(msg[1:len(msg)-2]) != nil {
					return serframe.None, nil
				}
				return serframe.Complete, nil
			}),
		),
	)
	m.ExitC = m.readMgr.ExitC
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

func (m *Conn) Name() string {
	return "rtu"
}

func (m *Conn) Device() interface{} {
	return m.conn
}

func (m *Conn) MsgWriter() (w io.Writer) {
	b := m.buf.w
	b.Reset()
	m.h.Reset()
	return io.MultiWriter(b, m.h)
}

var localEchoSetByEnv = os.Getenv("MODBUS_RTU_LOCAL_ECHO") == "1"

func (m *Conn) Send() (adu modbus.ADU, err error) {
	b := m.buf.w
	b.Write(m.h.Sum(nil))

	adu.PDUStart = 1
	adu.PDUEnd = -2
	adu.Bytes = b.Bytes()

	var opts []serframe.ReceptionOption
	if m.LocalEcho || localEchoSetByEnv {
		opts = append(opts, serframe.WithLocalEcho(adu.Bytes))
	}
	err = m.readMgr.StartReception(m.buf.r, opts...)
	if err != nil {
		return adu, err
	}

	_, err = b.WriteTo(m.conn)
	if err != nil {
		m.readMgr.CancelReception()
	}
	if port, ok := m.conn.(serport.Port); ok {
		err = port.Drain()
	}
	return adu, err
}

func (m *Conn) EnableReceive() error {
	return m.readMgr.StartReception(m.buf.r)
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
	adu.Bytes, err = m.readMgr.ReadFrame(ctx,
		serframe.WithInitialTimeout(tMax),
		serframe.WithExtInterByteTimeout(m.InterframeTimeout),
	)
	adu.PDUStart = 1
	adu.PDUEnd = -2
	if err != nil {
		err = mapErrors(err)
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

func (m *Conn) Stream() *serframe.Stream {
	return m.readMgr
}

var errorsMap = map[error]error{
	serframe.ErrTimeout:        modbus.ErrTimeout,
	serframe.ErrEchoMismatch:   modbus.ErrEchoMismatch,
	serframe.ErrInvalidEchoLen: modbus.ErrInvalidEchoLen,
	serframe.ErrOverflow:       modbus.ErrMaxRespLenExceeded,
}

func mapErrors(err error) error {
	modErr, ok := errorsMap[err]
	if !ok {
		return err
	}
	return modErr
}
