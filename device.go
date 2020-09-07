package modbus

type Device interface {
	Request(fn uint8, req Request, resp Response, opts ...ReqOption) error
}

type StdRegisterFuncs interface {
	ReadHoldingRegs(start uint16, data interface{}, opts ...ReqOption) error
	ReadInputRegs(start uint16, data interface{}, opts ...ReqOption) error
	WriteRegs(start uint16, data interface{}, opts ...ReqOption) error
}

type addressedDevice struct {
	addr byte
	bus  Bus
}

func newAddressedDevice(bus Bus) *addressedDevice {
	a := new(addressedDevice)
	a.bus = bus
	return a
}

func (d *addressedDevice) Request(fn uint8, req Request, resp Response, opts ...ReqOption) error {
	return d.bus.Request(d.addr, fn, req, resp, opts...)
}

type DeviceTestFunc func(addr byte, d Device) error

func ScanDevices(bus Bus, addrMin, addrMax byte, test DeviceTestFunc) (err error) {
	d := newAddressedDevice(bus)
	for a := addrMin; a <= addrMax; a++ {
		d.addr = byte(a)
		err = test(d.addr, d)
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
