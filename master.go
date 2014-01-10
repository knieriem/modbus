package modbus

import (
	"encoding/binary"
	"errors"
	"io"
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
		s = "Acknowledge"
	case XSlaveBusy:
		s = "slave busy"
	default:
		s = "unknown exception"
	}
	return
}

type TransmissionMode interface {
	Name() string
	MsgWriter() io.Writer
	Send() ([]byte, error)
	Receive(timeout time.Duration) (buf, msg []byte, err error)
}

type Stack struct {
	mode TransmissionMode

	Tracef          func(format string, a ...interface{})
	ResponseTimeout time.Duration
	TurnaroundDelay time.Duration
}

func NewStack(mode TransmissionMode) (stk *Stack) {
	stk = new(Stack)
	stk.mode = mode
	stk.ResponseTimeout = 1000 * time.Millisecond
	stk.TurnaroundDelay = 200 * time.Millisecond
	return
}

var ErrTimeout = errors.New("timeout")
var ErrCorruptMsgLen = errors.New("corrupt message length")
var ErrMsgTooShort = errors.New("msg too short")

func (stk *Stack) Request(addr, fn uint8, req Request, resp Response) (err error) {
	w := stk.mode.MsgWriter()

	w.Write([]byte{addr, fn})

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

	buf, msg, err := stk.mode.Receive(stk.ResponseTimeout)
	if stk.Tracef != nil {
		if err != nil {
			stk.Tracef("-> %s % x error: %v\n", stk.mode.Name(), buf, err)
			return
		} else {
			stk.Tracef("-> %s % x\n", stk.mode.Name(), buf)
		}
	}
	if err != nil {
		return
	}
	if msg[0] != addr {
		err = errors.New("response: addr mismatch")
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
		err = errors.New("response: function mismatch")
		return
	}
	if resp != nil {
		err = resp.Decode(msg)
	}
	return
}

type Request interface {
	Encode(io.Writer) error
}

type Response interface {
	Decode([]byte) error
}
