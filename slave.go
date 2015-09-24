package modbus

type Slave interface {
	Request(fn uint8, req Request, resp Response, expectedLengths []int) error
}

type StdRegisterFuncs interface {
	ReadHoldingRegs(start uint16, data interface{}) error
	ReadInputRegs(start uint16, data interface{}) error
	WriteRegs(start uint16, data interface{}) error
}
