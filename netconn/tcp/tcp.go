package tcp

import (
	"net"

	"modbus/modtcp"
	"modbus/netconn"
)

const (
	ModbusTCPPort = "502"
)

func init() {
	netconn.RegisterProtocol(&netconn.Proto{
		Name:           "tcp",
		OptionalFields: netconn.FieldAddr,
		AddrType:       []string{"ip"},
		Dial:           dial,
	})
}

func dial(cf *netconn.Conf) (conn *netconn.Conn, err error) {
	addr, err := cf.Addr.Complete(ModbusTCPPort)
	if err != nil {
		return
	}
	tc, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	nc := modtcp.NewNetConn(tc)
	conn = &netconn.Conn{
		Addr:    cf.MakeAddr(addr, false),
		NetConn: nc,
		Closer:  tc,
		ExitC:   nc.ExitC,
	}
	return
}
