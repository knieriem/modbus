package rtu

import (
	"io"
	"time"

	"github.com/knieriem/io/stream"
	"github.com/knieriem/modbus/netconn"
	"github.com/knieriem/modbus/rtu"
)

var InterframeTimeout struct {
	AdjustFunc func(*rtu.Conn)
}

var ConnWrapper stream.Wrapper

func init() {
	netconn.RegisterProtocol(&netconn.Proto{
		Name:           "rtu",
		OptionalFields: netconn.DevFields,
		Dial:           dial,
		InterfaceGroup: &serialPorts,
	})
}

func dial(cf *netconn.Conf) (conn *netconn.Conn, err error) {
	var f io.ReadWriteCloser
	var name string

	supportsOptions := true
	if cmd, match := parseCommand(cf.Device); match {
		f, err = cmd.Dial()
		name = cf.Device
		supportsOptions = false
	} else {
		f, name, err = openPort(cf)
	}
	if err != nil {
		return
	}

	f = stream.InheritCloser(ConnWrapper.Wrap(f, name), f)
	nc := rtu.NewNetConn(f)
	nc.OnReceiveError = func(m *rtu.Conn, err error) {
		if rtu.MaybeTruncatedMsg(err) {
			if InterframeTimeout.AdjustFunc != nil {
				InterframeTimeout.AdjustFunc(nc)
			}
			// delay execution so that a probably just arriving
			// tail of the message gets discarded
			time.Sleep(10 * time.Millisecond)
		}
	}
	nc.LocalEcho = cf.LocalEcho

	conn = &netconn.Conn{
		Addr:       cf.MakeAddr(name, supportsOptions),
		DeviceName: name,
		DeviceInfo: portInfo(name),
		NetConn:    nc,
		Closer:     f,
		ExitC:      nc.ExitC,
	}
	return
}
