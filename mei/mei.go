// Package mei implements the Modbus Encapsulated Interface Transport
package mei

import (
	"io"

	"modbus"
)

type Transport struct {
	typ     byte
	respBuf msg
	sl      modbus.Slave
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

func (t *Transport) Request(req []byte) (resp []byte, err error) {
	err = t.sl.Request(0x2B, &msg{typ: t.typ, data: req}, &t.respBuf, nil)
	if err != nil {
		return
	}
	resp = t.respBuf.data
	return
}

func NewTransport(sl modbus.Slave, meiType byte) *Transport {
	m := new(Transport)
	m.typ = meiType
	m.sl = sl
	m.respBuf.typ = meiType
	return m
}
