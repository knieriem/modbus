package rtu

import (
	"io"
	"time"

	"modbus"
)

type ReadMgr struct {
	buf     []byte
	req     chan []byte
	done    chan readResult
	Forward io.Writer
}

type ReadFunc func() ([]byte, error)

func NewReadMgr(rf ReadFunc, exitC chan<- int) *ReadMgr {
	m := new(ReadMgr)
	m.buf = make([]byte, 0, 64)
	m.req = make(chan []byte)
	m.done = make(chan readResult)
	go m.handle(rf, exitC)
	return m
}

func (m *ReadMgr) Start() {
	m.buf = m.buf[:0]
	m.req <- m.buf
}

func (m *ReadMgr) Cancel() {
	m.req <- nil
}

func (m *ReadMgr) Read(tMax, interframeTimeout time.Duration) (buf []byte, err error) {
	timeout := time.After(tMax)

readLoop:
	for {
		select {
		case r := <-m.done:
			m.buf = r.data
			if r.err != nil {
				err = r.err
				return
			}
			if interframeTimeout == 0 {
				break readLoop
			}
			timeout = time.After(interframeTimeout)

		case <-timeout:
			break readLoop
		}
	}
	m.req <- nil

	buf = m.buf
	if len(buf) == 0 {
		err = modbus.ErrTimeout
	}
	return
}

type readResult struct {
	data []byte
	err  error
}

func (m *ReadMgr) handle(read ReadFunc, exitC chan<- int) {
	var dest []byte

	data := make(chan readResult)
	go func() {
		exitCode := 1
		for {
			buf, err := read()
			data <- readResult{buf, err}
			<-data
			if err != nil {
				if err == io.EOF {
					exitCode = 0
				}
				break
			}
		}
		close(data)
		exitC <- exitCode
		return
	}()

loop:
	for {
		select {
		case dest = <-m.req:
		case r := <-data:
			if dest != nil {
				if r.err == nil {
					dest = append(dest, r.data...)
				}
			} else if m.Forward != nil {
				m.Forward.Write(r.data)
			}
			data <- readResult{}
			if dest != nil {
				select {
				case m.done <- readResult{dest, r.err}:
				case b := <-m.req:
					if m.Forward != nil {
						m.Forward.Write(dest)
					}
					dest = b
				}
			}
			if r.err != nil {
				break loop
			}
		}
	}
	close(m.done)
}
