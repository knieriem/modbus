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
		Dial:           dial,
		InterfaceGroup: &ipInterfaceGroup,
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

var ipInterfaceGroup = netconn.InterfaceGroup{
	Name:       "IP interfaces",
	Interfaces: ipInterfaces,
	Hidden:     true,
	Type:       "ip",
}

func ipInterfaces() (list []netconn.Interface) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		for _, addr := range addrs {
			switch a := addr.(type) {
			case *net.IPNet:
				desc := i.Name
				if len(i.HardwareAddr) != 0 {
					desc += ", hw=" + i.HardwareAddr.String()
				}
				list = append(list, netconn.Interface{
					Name: a.String(),
					Desc: desc,
					Elem: a,
				})
			}
		}
	}
	return
}
