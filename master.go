package modbus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"time"
)

var ByteOrder = binary.BigEndian

const (
	ErrorMask = 0x80
)

type Exception uint8

const (
	XIllegalFunc Exception = 1 + iota
	XIllegalDataAddr
	XIllegalDataVal
	XSlaveDeviceFailure
	XACK
	XSlaveBusy
	_
	XMemoryParityError
	_
	XGwPathUnavail
	XGwTargetFailedToRespond
)

func (x Exception) Error() (s string) {
	switch x {
	case XIllegalFunc:
		s = "illegal function"
	case XIllegalDataAddr:
		s = "illegal data addr"
	case XIllegalDataVal:
		s = "illegal data value"
	case XSlaveDeviceFailure:
		s = "slave device failure"
	case XACK:
		s = "acknowledge"
	case XSlaveBusy:
		s = "slave device busy"
	case XMemoryParityError:
		s = "memory parity error"
	case XGwPathUnavail:
		s = "gateway path unavailable"
	case XGwTargetFailedToRespond:
		s = "gateway target device failed to respond"
	default:
		s = "unknown exception 0x" + strconv.FormatUint(uint64(x), 16)
	}
	return
}

type NetConn interface {
	Name() string
	MsgWriter() io.Writer
	Send() ([]byte, error)
	Receive(timeout time.Duration, verifyLen func(int) error) (buf, msg []byte, err error)
}

type Stack struct {
	mode NetConn

	Tracef          func(format string, a ...interface{})
	ResponseTimeout time.Duration
	TurnaroundDelay time.Duration
	RequestStats    RequestStats
}

func NewStack(mode NetConn) (stk *Stack) {
	stk = new(Stack)
	stk.mode = mode
	stk.ResponseTimeout = 1000 * time.Millisecond
	stk.TurnaroundDelay = 4 * time.Millisecond
	return
}

type Error string

func (e Error) Error() string {
	return "modbus: " + string(e)
}

var ErrTimeout = Error("timeout")
var ErrEchoMismatch = Error("local echo mismatch")
var ErrUnexpectedEcho = Error("unexpected echo")
var ErrInvalidEchoLen = Error("invalid local echo length")
var ErrMaxReqLenExceeded = Error("max request length exceeded")
var ErrCRC = Error("CRC error")

type InvalidMsgLenError struct {
	Len         int
	ExpectedLen []int
}

func NewInvalidPayloadLen(have int, want ...int) error {
	for i := range want {
		want[i] += 2
	}
	return &InvalidMsgLenError{Len: 2 + have, ExpectedLen: want}
}

func NewInvalidMsgLen(have int, want ...int) error {
	return &InvalidMsgLenError{Len: have, ExpectedLen: want}
}

func NewLengthFieldMismatch(lengthField int, msgLen int) error {
	return fmt.Errorf("length field value (%d) and msg length inconsistent (%d)", lengthField, msgLen)
}

func NewInvalidUserBufLen(have int, want int) error {
	return fmt.Errorf("length of user provided buffer (%d), and message length (%d) inconsitent", have, want)
}

func (e InvalidMsgLenError) Error() string {
	if e.TooLong() {
		return fmt.Sprintf("msg too long (have %d, want %d)", e.Len, e.ExpectedLen[0])
	}
	if e.TooShort() {
		return fmt.Sprintf("msg too short (have %d, want %d)", e.Len, e.ExpectedLen[0])
	}
	return fmt.Sprintf("invalid msg length (have %d, want &v)", e.Len, e.ExpectedLen)
}

func (e *InvalidMsgLenError) TooLong() bool {
	return len(e.ExpectedLen) == 1 && e.Len > e.ExpectedLen[0]
}

func (e *InvalidMsgLenError) TooShort() bool {
	return len(e.ExpectedLen) == 1 && e.Len < e.ExpectedLen[0]
}

type Bus interface {
	Request(addr, fn uint8, req Request, resp Response, expectedLengths []int) error
}

func (stk *Stack) Request(addr, fn uint8, req Request, resp Response, expectedLengths []int) (err error) {
	w := stk.mode.MsgWriter()
	var msgLen msgLenCounter
	mw := io.MultiWriter(&msgLen, w)
	mw.Write([]byte{addr, fn})

	defer func() {
		stk.RequestStats.Update(err)
	}()
	if req != nil {
		err = req.Encode(mw)
		if err != nil {
			return
		}
	}
	if msgLen > 254 {
		return ErrMaxReqLenExceeded
	}

	sent, err := stk.mode.Send()
	if err != nil {
		return
	}
	if stk.Tracef != nil {
		stk.Tracef("<- %s [%d] % x\n", stk.mode.Name(), len(sent), sent)
	}
	if addr == 0 {
		time.Sleep(stk.TurnaroundDelay)
		return
	}

	verifyRespLen := func(n int) error {
		return verifyMsgLength(n, expectedLengths)
	}

	buf, msg, err := stk.mode.Receive(stk.ResponseTimeout, verifyRespLen)
	if err != nil {
		if bytes.Equal(buf, sent) {
			err = ErrUnexpectedEcho
		}
	}
	if stk.Tracef != nil {
		if err != nil {
			stk.Tracef("-> %s [%d] % x error: %v\n", stk.mode.Name(), len(buf), buf, err)
			return
		}
		stk.Tracef("-> %s [%d] % x\n", stk.mode.Name(), len(buf), buf)
	}
	if err != nil {
		return
	}
	err = verifyRespLen(len(msg) - 2)
	if err != nil {
		return
	}
	if msg[0] != addr {
		err = Error("response: addr mismatch")
		return
	}
	if msg[1] == ErrorMask|fn {
		// handle error
		if len(msg) != 3 {
			err = NewInvalidMsgLen(len(msg), 3)
			return
		}
		err = Exception(msg[2])
		return
	}
	if msg[1] != fn {
		err = Error("response: function mismatch")
		return
	}
	if resp != nil {
		err = resp.Decode(msg)
	}
	return
}

type msgLenCounter int

func (lc *msgLenCounter) Write(data []byte) (int, error) {
	*lc += msgLenCounter(len(data))
	return len(data), nil
}

func verifyMsgLength(n int, valid []int) (err error) {
	if valid == nil {
		return
	}
	if n == 1 {
		return // might be an error response
	}
	max := 0
	for i, l := range valid {
		if i > 0 && l == 0 {
			return // any length allowed
		}
		if l == n {
			return
		}
		if l > max {
			max = l
		}
	}
	if n > max {
		err = NewInvalidPayloadLen(n, max)
	} else {
		err = NewInvalidPayloadLen(n, valid...)
	}
	return
}

func MsgInvalid(err error) bool {
	if _, ok := err.(InvalidMsgLenError); ok {
		return true
	}
	switch err {
	default:
		return false
	case ErrInvalidEchoLen:
	case XGwTargetFailedToRespond:
	case ErrCRC:
	}
	return true
}

type Request interface {
	Encode(io.Writer) error
}

type Response interface {
	Decode([]byte) error
}
