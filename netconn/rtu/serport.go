package rtu

import (
	"io"
	"strings"

	"modbus/netconn"

	"github.com/knieriem/serport"
	"github.com/knieriem/serport/serenum"
)

func portInfo(name string) string {
	return serenum.Lookup(name).Format(nil)
}

func openPort(cf *netconn.Conf) (c io.ReadWriteCloser, portName string, err error) {
	inictl := strings.Join(cf.Options, " ")

	port, portName, err := serport.Choose(cf.Device, inictl)
	if err == nil {
		c = port
	}
	return
}

var serialPorts = netconn.InterfaceGroup{
	Name:       "Serial ports",
	Interfaces: serialInterfaces,
	SortPrefix: "A01",
	Type:       "serport",
}

func serialInterfaces() (list []netconn.Interface) {
	for _, info := range serenum.Ports() {
		list = append(list, netconn.Interface{
			Name: info.Device,
			Desc: info.Format(nil),
			Elem: info,
		})
	}
	return
}
