package modbus

type Slave interface {
	Request(fn uint8, req Request, resp Response, opts ...ReqOption) error
}

type StdRegisterFuncs interface {
	ReadHoldingRegs(start uint16, data interface{}) error
	ReadInputRegs(start uint16, data interface{}) error
	WriteRegs(start uint16, data interface{}) error
}

type addressedSlave struct {
	addr byte
	bus  Bus
}

func newAddressedSlave(bus Bus) *addressedSlave {
	a := new(addressedSlave)
	a.bus = bus
	return a
}

func (sl *addressedSlave) Request(fn uint8, req Request, resp Response, opts ...ReqOption) error {
	return sl.bus.Request(sl.addr, fn, req, resp, opts...)
}

type SlaveTestFunc func(addr byte, sl Slave) error

func ScanSlaves(bus Bus, addrMin, addrMax byte, test SlaveTestFunc) (err error) {
	sl := newAddressedSlave(bus)
	for a := addrMin; a <= addrMax; a++ {
		sl.addr = byte(a)
		err = test(sl.addr, sl)
		if err != nil {
			if err == ErrTimeout || MsgInvalid(err) {
				err = nil
				continue
			}
			break
		}
	}
	return
}
