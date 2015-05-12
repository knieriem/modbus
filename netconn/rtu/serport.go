package rtu

import (
	"errors"
	"fmt"
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
	var port serport.Port

	portName = cf.Device
	inictl := strings.Join(cf.Options, " ")

	if portName != "" {
		goto withName
	}
	portName, err = choosePort()
	if err != nil {
		return
	}

withName:
	port, err = serport.Open(portName, inictl)
	if err == nil {
		c = port
	}
	return
}

func choosePort() (name string, err error) {
	sep := "-"
	list := serenum.Ports()
	switch len(list) {
	case 0:
		fmt.Print("Enter serial port: ")
		_, err = fmt.Scanln(&name)
		return
	case 1:
		name = list[0].Device
		return
	case 2:
		sep = ", "
	}
	fmt.Println("\nChoose a serial port: ")
	for i, p := range list {
		fmt.Printf("  %d\t%v (%v)\n", i, p.Device, p.Format(nil))
	}
	fmt.Print("Enter a number (0", sep, len(list)-1, "): ")

	var i int
	_, err = fmt.Scanln(&i)
	if err == nil {
		if i < len(list) {
			name = list[i].Device
		} else {
			err = errors.New("value exceeds maximum index")
		}
	}
	fmt.Print("\n")
	return
}
