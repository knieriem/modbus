// Package mei implements the Modbus Encapsulated Interface Transport
package mei

import (
	"io"

	"github.com/knieriem/modbus"
)

type Transport struct {
	typ     byte
	respBuf msg
	dev     modbus.Device
}

type msg struct {
	typ  byte
	data []byte
}

func (m *msg) Encode(w io.Writer) (err error) {
	_, err = w.Write([]byte{m.typ})
	if err != nil {
		return
	}
	_, err = w.Write(m.data)
	return
}

func (m *msg) Decode(buf []byte) (err error) {
	buf = buf[2:]
	if len(buf) < 1 {
		return modbus.NewInvalidPayloadLen(len(buf), 1)
	}
	if buf[0] != m.typ {
		return modbus.Error("wrong MEI code")
	}
	data := buf[1:]
	n := len(data)
	if cap(m.data) < n {
		m.data = make([]byte, n)
	} else {
		m.data = m.data[:n]
	}
	copy(m.data, data)
	return
}

func (t *Transport) Request(req []byte, opts ...modbus.ReqOption) (resp []byte, err error) {
	err = t.dev.Request(0x2B, &msg{typ: t.typ, data: req}, &t.respBuf, opts...)
	if err != nil {
		return
	}
	resp = t.respBuf.data
	return
}

func NewTransport(dev modbus.Device, meiType byte) *Transport {
	m := new(Transport)
	m.typ = meiType
	m.dev = dev
	m.respBuf.typ = meiType
	return m
}
