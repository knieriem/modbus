package rtu

import (
	"io"
	"strings"

	"github.com/knieriem/modbus/netconn"

	"github.com/knieriem/serport"
	"github.com/knieriem/serport/serenum"
)

func portInfo(name string) string {
	return serenum.Lookup(name).Format(nil)
}

func openPort(cf *netconn.Conf) (c io.ReadWriteCloser, portName string, err error) {
	inictl := strings.Join(cf.Options, " ")

	portName, err = serport.Choose(cf.Device)
	if err != nil {
		return nil, "", err
	}
	port, err := serport.Open(portName, serport.MergeCtlCmds(serport.StdConf, inictl))
	if err != nil {
		return nil, portName, err
	}
	return port, portName, nil
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
