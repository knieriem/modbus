package modbus

import (
	"encoding/binary"
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
	stk.TurnaroundDelay = 200 * time.Millisecond
	return
}

type Error string

func (e Error) Error() string {
	return "modbus: " + string(e)
}

var ErrTimeout = Error("timeout")
var ErrCorruptMsgLen = Error("corrupt msg length")
var ErrInvalidMsgLen = Error("invalid msg length")
var ErrMsgTooShort = Error("msg too short")
var ErrMsgTooLong = Error("msg too long")
var ErrCRC = Error("CRC error")

type Bus interface {
	Request(addr, fn uint8, req Request, resp Response, expectedLengths []int) error
}

func (stk *Stack) Request(addr, fn uint8, req Request, resp Response, expectedLengths []int) (err error) {
	w := stk.mode.MsgWriter()

	w.Write([]byte{addr, fn})

	defer func() {
		stk.RequestStats.Update(err)
	}()
	if req != nil {
		err = req.Encode(w)
		if err != nil {
			return
		}
	}

	buf, err := stk.mode.Send()
	if err != nil {
		return
	}
	if stk.Tracef != nil {
		stk.Tracef("<- %s % x\n", stk.mode.Name(), buf)
	}
	if addr == 0 {
		time.Sleep(stk.TurnaroundDelay)
		return
	}

	verifyRespLen := func(n int) error {
		return verifyMsgLength(n, expectedLengths)
	}
	buf, msg, err := stk.mode.Receive(stk.ResponseTimeout, verifyRespLen)
	if stk.Tracef != nil {
		if err != nil {
			stk.Tracef("-> %s % x error: %v\n", stk.mode.Name(), buf, err)
			return
		}
		stk.Tracef("-> %s % x\n", stk.mode.Name(), buf)
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
			err = ErrCorruptMsgLen
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
		err = ErrMsgTooLong
	} else {
		err = ErrInvalidMsgLen
	}
	return
}

func MsgInvalid(err error) bool {
	switch err {
	default:
		return false
	case ErrMsgTooShort:
	case ErrMsgTooLong:
	case ErrCorruptMsgLen:
	case ErrInvalidMsgLen:
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
