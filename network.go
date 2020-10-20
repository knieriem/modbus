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
	XDeviceFailure
	XACK
	XDeviceBusy
	_
	XMemoryParityError
	_
	XGwPathUnavail
	XGwTargetFailedToRespond
)

func (x Exception) Error() string {
	var s string
	switch x {
	case XIllegalFunc:
		s = "illegal function"
	case XIllegalDataAddr:
		s = "illegal data addr"
	case XIllegalDataVal:
		s = "illegal data value"
	case XDeviceFailure:
		s = "device failure"
	case XACK:
		s = "acknowledge"
	case XDeviceBusy:
		s = "device busy"
	case XMemoryParityError:
		s = "memory parity error"
	case XGwPathUnavail:
		s = "gateway path unavailable"
	case XGwTargetFailedToRespond:
		s = "gateway target device failed to respond"
	default:
		s = "unknown exception 0x" + strconv.FormatUint(uint64(x), 16)
	}
	return "modbus: " + s
}

type NetConn interface {
	Name() string
	MsgWriter() io.Writer
	Send() ([]byte, error)
	Receive(timeout time.Duration, ls *ExpectedRespLenSpec) (ADU, error)
	Device() interface{}
}

type ADU struct {
	Bytes []byte

	// PDUStart contains the start index of a PDU,
	// relative to the start of an ADU
	PDUStart int

	// PDUStart contains the end index of a PDU,
	// relative to the end of an ADU.
	PDUEnd int
}

// AddrPDU returns the address and the PDU parts of
// an ADU. It assumes that the address always is in the
// byte before the PDU.
// AddrPDU returns zero and nil in case the length of
// the ADU is too small.
func (adu *ADU) AddrPDU() (uint8, []byte) {
	if adu.PDUStart == 0 {
		return 0, nil
	}
	end := len(adu.Bytes) + adu.PDUEnd
	if end < adu.PDUStart {
		return 0, nil
	}
	addr := adu.Bytes[adu.PDUStart-1]

	// Set the capacity of the PDU to the maximum possible value,
	// leaving space for an optional CRC. This way a function
	// handler may decide whether the slice may be used to
	// store a response.
	pdu := adu.Bytes[adu.PDUStart : end : cap(adu.Bytes)+adu.PDUEnd]
	return addr, pdu
}

type Network struct {
	conn NetConn

	Tracef          func(format string, a ...interface{})
	ResponseTimeout time.Duration
	TurnaroundDelay time.Duration
	RequestStats    RequestStats
}

func NewNetwork(conn NetConn) (netw *Network) {
	netw = new(Network)
	netw.conn = conn
	netw.ResponseTimeout = 1000 * time.Millisecond
	netw.TurnaroundDelay = 4 * time.Millisecond
	return
}

func (netw *Network) Device() interface{} {
	return netw.conn.Device()
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

type MismatchError struct {
	Req     MsgHdr
	Resp    MsgHdr
	origErr error
}

func (e *MismatchError) Error() string {
	var s string
	if e.Req[0] != e.Resp[0] {
		s = "addr"
	} else {
		s = "fn code"
	}
	return fmt.Sprintf("modbus: %s mismatch (expected: %v, got: %v)", s, e.Req, e.Resp)
}

func (e *MismatchError) Unwrap() error {
	return e.origErr
}

type MsgHdr [2]byte

func (h MsgHdr) String() string {
	return fmt.Sprintf("% x", h[:])
}

func (h MsgHdr) matchAddr(h2 MsgHdr) bool {
	return h[0] == h2[0]
}
func (h MsgHdr) matchFn(h2 MsgHdr) bool {
	return h[1] == h2[1] || (ErrorMask|h[1]) == h2[1]
}

type InvalidLenError struct {
	MsgContext
	Len         int
	ExpectedLen []int
}

type MsgContext string

const (
	MsgContextADU  MsgContext = "ADU"
	MsgContextPDU  MsgContext = "PDU"
	MsgContextData MsgContext = "data part"
)

func NewInvalidLen(ctx MsgContext, have int, want ...int) error {
	return &InvalidLenError{MsgContext: ctx, Len: have, ExpectedLen: want}
}

func NewLengthFieldMismatch(lengthField int, dataLen int) error {
	return fmt.Errorf("modbus: length field value (%d) and actual data length inconsistent (%d)", lengthField, dataLen)
}

func NewInvalidUserBufLen(have int, want int) error {
	return fmt.Errorf("modbus: length of user provided buffer (%d), and data length (%d) inconsistent", have, want)
}

func (e InvalidLenError) Error() string {
	if e.MsgContext == "" {
		return fmt.Sprint("modbus: invalid length (unspecified)")
	}
	if e.TooLong() {
		return fmt.Sprintf("modbus: %s too long (have %d, want %d)", e.MsgContext, e.Len, e.ExpectedLen[0])
	}
	if e.TooShort() {
		return fmt.Sprintf("modbus: %s too short (have %d, want %d)", e.MsgContext, e.Len, e.ExpectedLen[0])
	}
	return fmt.Sprintf("modbus: invalid %s length (have %d, want %v)", e.MsgContext, e.Len, e.ExpectedLen)
}

func (e *InvalidLenError) TooLong() bool {
	return len(e.ExpectedLen) == 1 && e.Len > e.ExpectedLen[0]
}

func (e *InvalidLenError) TooShort() bool {
	return len(e.ExpectedLen) == 1 && e.Len < e.ExpectedLen[0]
}

type Bus interface {
	Request(addr, fn uint8, req Request, resp Response, opts ...ReqOption) error
}

type ReqOption func(*reqOptions)

type reqOptions struct {
	timeout                time.Duration
	timeoutIncr            time.Duration
	waitFull               bool
	nRetriesOnTimeout      int
	nRetriesOnInvalidReply int
	retryDelay             time.Duration
	expectedLenSpec        *ExpectedRespLenSpec
}

// ExpectedRespLen is a request option that specifies
// which PDU size is expected for a fixed length response.
// It allows the request procedure to return as early as possible.
func ExpectedRespLen(n int) ReqOption {
	return func(r *reqOptions) {
		r.expectedLenSpec = &ExpectedRespLenSpec{ValidLen: []int{n}}
	}
}

func ExpectedRespLengths(l []int) ReqOption {
	return func(r *reqOptions) {
		r.expectedLenSpec = &ExpectedRespLenSpec{ValidLen: l}
	}
}

func WithTimeout(d time.Duration) ReqOption {
	return func(r *reqOptions) {
		r.timeout = d
	}
}

func WaitFull() ReqOption {
	return func(r *reqOptions) {
		r.waitFull = true
	}
}

func RetryOnTimeout(n int, timeoutIncr time.Duration) ReqOption {
	return func(r *reqOptions) {
		r.nRetriesOnTimeout = n
		r.timeoutIncr = timeoutIncr
	}
}

func RetryOnInvalidReply(n int, retryDelay time.Duration) ReqOption {
	return func(r *reqOptions) {
		r.nRetriesOnInvalidReply = n
		r.retryDelay = retryDelay
	}
}

type ExpectedRespLenSpec struct {
	ValidLen []int
	Variable *VariableRespLenSpec
}

// VariableRespLenSpec defines how a PDU size
// of a variable-length response can be verified,
// e.g. in case of requests like Read Device Identification.
// It allows the request procedure to return as early as possible.
type VariableRespLenSpec struct {
	PrefixLen     int
	NumItemsFixed int
	NumItemsIndex int
	ItemLenIndex  int
	ItemTailLen   int
	TailLen       int
}

// VariableRespLen is a request option that defines
// a VariableRespLenSpec to be used during the request.
func VariableRespLen(vs *VariableRespLenSpec) ReqOption {
	return func(r *reqOptions) {
		r.expectedLenSpec = &ExpectedRespLenSpec{Variable: vs}
	}
}

func (netw *Network) Request(addr, fn uint8, req Request, resp Response, opts ...ReqOption) (err error) {
	var rqo reqOptions
	rqo.timeout = netw.ResponseTimeout
	if i, ok := resp.(interface{ ExpectedLenSpec() *ExpectedRespLenSpec }); ok {
		rqo.expectedLenSpec = i.ExpectedLenSpec()
	}
	for _, o := range opts {
		o(&rqo)
	}
	defer func() {
		netw.RequestStats.Update(err)
	}()

	nRetries := 0
retry:
	w := netw.conn.MsgWriter()
	var msgLen msgLenCounter
	mw := io.MultiWriter(&msgLen, w)
	mw.Write([]byte{addr, fn})
	if req != nil {
		err = req.Encode(mw)
		if err != nil {
			return
		}
	}
	if msgLen > 254 {
		return ErrMaxReqLenExceeded
	}

	sent, err := netw.conn.Send()
	if err != nil {
		return
	}
	if netw.Tracef != nil {
		netw.Tracef("<- %s [%d] % x\n", netw.conn.Name(), len(sent), sent)
	}
	if addr == 0 {
		time.Sleep(netw.TurnaroundDelay)
		return
	}

	if rqo.waitFull {
		t0 := time.Now()
		defer func() {
			remain := t0.Add(rqo.timeout).Sub(time.Now())
			if remain > 0 {
				time.Sleep(remain)
			}
		}()
	}

	adu, err := netw.conn.Receive(rqo.timeout, rqo.expectedLenSpec)

	buf := adu.Bytes
	respAddr, pdu := adu.AddrPDU()

	if len(pdu) >= 1 {
		want := MsgHdr{addr, fn}
		have := MsgHdr{respAddr, pdu[0]}
		if !want.matchAddr(have) || !want.matchFn(have) {
			err = &MismatchError{Req: want, Resp: have, origErr: err}
		} else if err != nil {
			if bytes.Equal(buf, sent) {
				err = ErrUnexpectedEcho
			}
		}
	}
	if netw.Tracef != nil {
		if err != nil {
			netw.Tracef("-> %s [%d] % x error: %v\n", netw.conn.Name(), len(buf), buf, err)
			return
		}
		netw.Tracef("-> %s [%d] % x\n", netw.conn.Name(), len(buf), buf)
	}
	if err != nil {
		if err == ErrTimeout {
			if nRetries < rqo.nRetriesOnTimeout {
				nRetries++
				rqo.timeout += rqo.timeoutIncr
				goto retry
			}
		} else if nRetries < rqo.nRetriesOnInvalidReply {
			if MsgInvalid(err) {
				if rqo.retryDelay > 0 {
					time.Sleep(rqo.retryDelay)
				}
				nRetries++
				goto retry
			}
		}
		return err
	}
	if ls := rqo.expectedLenSpec; ls != nil {
		err = ls.CheckLen(pdu)
		if err != nil {
			if bytes.Equal(buf, sent) {
				err = ErrUnexpectedEcho
			}
			return
		}
	}
	if pdu[0] == ErrorMask|fn {
		// handle error
		if len(pdu) != 2 {
			err = NewInvalidLen(MsgContextPDU, len(pdu), 2)
			return
		}
		err = Exception(pdu[1])
		return
	}
	if resp != nil {
		err = resp.Decode(pdu[1:])
	}
	return
}

type msgLenCounter int

func (lc *msgLenCounter) Write(data []byte) (int, error) {
	*lc += msgLenCounter(len(data))
	return len(data), nil
}

func (ls *ExpectedRespLenSpec) CheckLen(pdu []byte) error {
	if ls == nil {
		return nil
	}

	n := len(pdu)
	if n == 2 {
		if pdu[0]&0x80 != 0 {
			return nil // is an exception response
		}
	}

	valid := ls.ValidLen
	if valid == nil {
		if v := ls.Variable; v != nil {
			expectedLen, ok := v.Match(pdu)
			if !ok {
				return NewInvalidLen(MsgContextData, n, expectedLen)
			}
		}
		return nil
	}
	max := 0
	for i, l := range valid {
		if i > 0 && l == 0 {
			return nil // allow any length
		}
		if l == n {
			return nil
		}
		if l > max {
			max = l
		}
	}
	if n > max {
		return NewInvalidLen(MsgContextPDU, n, max)
	}
	return NewInvalidLen(MsgContextPDU, n, valid...)
}

func (v *VariableRespLenSpec) Match(pdu []byte) (expectedLen int, match bool) {
	n := len(pdu)
	nx := 0
	ni := v.NumItemsFixed
	if ni == 0 {
		nx += v.NumItemsIndex + 1
		if n < nx {
			return nx, false
		}
		ni = int(pdu[nx-1])
	}
	nx += v.PrefixLen
	if n < nx {
		return nx, false
	}
	for i := 0; i < ni; i++ {
		nx += v.ItemLenIndex + 1
		if n < nx {
			return nx, false
		}
		nx += int(pdu[nx-1]) + v.ItemTailLen
	}
	nx += v.TailLen
	return nx, n == nx
}

func MsgInvalid(err error) bool {
	if _, ok := err.(*InvalidLenError); ok {
		return true
	}
	if _, ok := err.(*MismatchError); ok {
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
	// Encode writes the data part of a PDU,
	// i.e. the PDU without the function code.
	Encode(io.Writer) error
}

type Response interface {
	// Decode works on the data part of a PDU,
	// i.e. the PDU without the function code.
	Decode([]byte) error
}
