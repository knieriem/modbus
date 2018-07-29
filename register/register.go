package register

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"

	"modbus"
)

type Slave struct {
	modbus.Slave
}

func NewSlave(sl modbus.Slave) *Slave {
	return &Slave{Slave: sl}
}

type Error string

func (e Error) Error() string {
	return "register: " + string(e)
}

type Func func(regAddr uint16, data interface{}) error

type readRegistersResp struct {
	numBytes byte
	buf      interface{}
}

func (r *readRegistersResp) Decode(buf []byte) (err error) {
	buf = buf[2:]
	if len(buf) < 1 {
		return modbus.NewInvalidPayloadLen(len(buf), 1)
	}
	r.numBytes = buf[0]
	data := buf[1:]
	if int(r.numBytes) != len(data) {
		return modbus.NewLengthFieldMismatch(int(r.numBytes), len(data))
	}
	err = binary.Read(bytes.NewReader(data), modbus.ByteOrder, r.buf)
	return
}

type readRegisters struct {
	Start uint16
	N     uint16
}

func (r *readRegisters) Encode(w io.Writer) (err error) {
	err = binary.Write(w, modbus.ByteOrder, r)
	return
}

func (sl *Slave) readRegs(fn uint8, startAddr uint16, dest interface{}) (err error) {
	var resp readRegistersResp

	nBytes, nReg, err := dataBufSize(dest)
	if err != nil {
		return
	}
	resp.buf = dest
	err = sl.Request(fn, &readRegisters{Start: startAddr, N: nReg}, &resp, []int{nBytes + 1})
	return
}

func (sl *Slave) ReadHoldingRegs(startReg uint16, dest interface{}) error {
	return sl.readRegs(3, startReg, dest)
}

func (sl *Slave) ReadInputRegs(startReg uint16, dest interface{}) error {
	return sl.readRegs(4, startReg, dest)
}

type singleReg struct {
	Addr  uint16
	Value uint16
}

func (r *singleReg) Encode(w io.Writer) (err error) {
	err = binary.Write(w, modbus.ByteOrder, r)
	return
}

func (sl *Slave) WriteReg(regAddr uint16, data interface{}) (err error) {
	var buf bytes.Buffer

	err = binary.Write(&buf, modbus.ByteOrder, data)
	if err != nil {
		return
	}
	if buf.Len() != 2 {
		err = errors.New("the size of the data buffer must be two bytes")
		return
	}
	value := modbus.ByteOrder.Uint16(buf.Bytes())
	err = sl.Request(6, &singleReg{Addr: regAddr, Value: value}, nil, []int{4})
	return
}

type multipleRegs struct {
	Addr   uint16
	NRegs  uint16
	NBytes uint8
	Values interface{}
}

type Encoder interface {
	Encode(io.Writer) error
}

func (r *multipleRegs) Encode(w io.Writer) (err error) {
	binary.Write(w, modbus.ByteOrder, r.Addr)
	binary.Write(w, modbus.ByteOrder, r.NRegs)
	binary.Write(w, modbus.ByteOrder, r.NBytes)
	if e, ok := r.Values.(Encoder); ok {
		err = e.Encode(w)
	} else {
		err = binary.Write(w, modbus.ByteOrder, r.Values)
	}
	return
}

func (sl *Slave) WriteRegs(startAddr uint16, data interface{}) (err error) {
	nBytes, nReg, err := dataBufSize(data)
	if err != nil {
		return
	}
	if nReg == 1 {
		err = sl.WriteReg(startAddr, data)
		return
	}
	err = sl.Request(0x10, &multipleRegs{Addr: startAddr, NRegs: nReg, NBytes: uint8(nBytes), Values: data}, nil, []int{4})
	return
}

func dataBufSize(data interface{}) (nBytes int, nReg uint16, err error) {
	n := binary.Size(data)
	if n == -1 {
		err = errors.New("data buffer not compatible with encoding/binary package")
		return
	}
	nBytes = n
	if (nBytes & 1) != 0 {
		err = errors.New("binary size does not equal a multiple of two")
		return
	}
	nReg = uint16(nBytes / 2)
	return
}

func parseOffset(expr string) (value string, offset int, err error) {
	if i := strings.IndexAny(expr, "+-"); i != -1 {
		i64, err := strconv.ParseInt(expr[i:], 0, 16)
		if err != nil {
			return "", 0, err
		}
		return expr[:i], int(i64), nil
	}
	return expr, 0, nil
}

func ParseAddr(addrStr string) (addr uint16, err error) {
	addrStr, offset, err := parseOffset(addrStr)
	if err != nil {
		return 0, err
	}
	u64, err := strconv.ParseUint(addrStr, 0, 16)
	if err != nil {
		return 0, err
	}
	return uint16(u64) + uint16(offset), nil
}

func ParseModiconNum(sl modbus.StdRegisterFuncs, value string) (addr uint16, f Func, err error) {
	value, offset, err := parseOffset(value)
	if err != nil {
		return 0, nil, err
	}
	if len(value) == 0 {
		return 0, nil, Error("empty register number")
	}

	// decode reference
	switch value[0] {
	case '3':
		f = sl.ReadInputRegs
	case '4':
		f = sl.ReadHoldingRegs
	case ' ', '\t':
		return 0, nil, Error("initial white-space not allowed")
	default:
		return 0, nil, Error("reference not suppored")
	}

	value = value[1:]
	u64, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, nil, err
	}
	if u64 == 0 {
		return 0, nil, Error("0 is not a valid register number")
	}
	u64 -= 1
	switch len(value) {
	default:
		return 0, nil, Error("invalid number of digits")
	case 5:
		if u64 > 0xFFFF {
			return 0, nil, Error("number exceeds address range")
		}
	case 4:
	}
	return uint16(u64) + uint16(offset), f, nil
}
