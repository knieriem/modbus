package rtu

import (
	"time"

	"modbus/netconn"
	"modbus/rtu"

	"te/stream"
)

var InterframeTimeout struct {
	AdjustFunc func(*rtu.Conn)
	Initial    time.Duration
}

var ConnWrapper stream.Wrapper

func init() {
	netconn.RegisterProtocol(&netconn.Proto{
		Name:           "rtu",
		OptionalFields: netconn.DevFields,
		AddrType:       []string{"serport"},
		Dial:           dial,
	})
}

func dial(cf *netconn.Conf) (conn *netconn.Conn, err error) {
	f, name, err := openPort(cf)
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
	t := nc.InterframeTimeout
	if tifr := InterframeTimeout.Initial; tifr != 0 {
		t = tifr
	}
	nc.InterframeTimeout = t

	conn = &netconn.Conn{
		Addr:       cf.MakeAddr(name, true),
		Device:     name,
		DeviceInfo: portInfo(name),
		NetConn:    nc,
		Closer:     f,
		ExitC:      nc.ExitC,
	}
	return
}
