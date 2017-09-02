package rtu

import (
	"io"
	"time"

	"modbus"
)

type ReadMgr struct {
	buf         []byte
	req         chan []byte
	done        chan readResult
	errC        chan error
	eof         bool
	Forward     io.Writer
	MsgComplete func([]byte) bool
	IntrC       <-chan error
	checkBytes  func([]byte) bool
}

type ReadFunc func() ([]byte, error)

func NewReadMgr(rf ReadFunc, exitC chan<- int) *ReadMgr {
	m := new(ReadMgr)
	m.buf = make([]byte, 0, 64)
	m.req = make(chan []byte)
	m.done = make(chan readResult)
	m.errC = make(chan error)
	go m.handle(rf, exitC)
	return m
}

func (m *ReadMgr) Start() (err error) {
	if m.eof {
		err = io.EOF
		return
	}
	select {
	case err = <-m.errC:
		m.eof = true
	default:
		m.buf = m.buf[:0]
		m.req <- m.buf
	}
	return
}

func (m *ReadMgr) Cancel() {
	m.req <- nil
}

func (m *ReadMgr) Read(tMax, interframeTimeout time.Duration) (buf []byte, err error) {
	if m.eof {
		err = io.EOF
		return
	}
	bufok := false
	if m.checkBytes == nil {
		bufok = true
	}
	nto := 0
	timeout := time.NewTimer(tMax)
readLoop:
	for {
		select {
		case r := <-m.done:
			nb := len(m.buf)
			m.buf = r.data
			if m.checkBytes != nil {
				bufok = m.checkBytes(m.buf[nb:])
			}
			if r.err != nil {
				err = r.err
				close(m.req)
				m.eof = true
				return
			}
			if m.MsgComplete != nil {
				if m.MsgComplete(m.buf) {
					if !timeout.Stop() {
						<-timeout.C
					}
					break readLoop
				}
			}
			if interframeTimeout == 0 {
				if !timeout.Stop() {
					<-timeout.C
				}
				break readLoop
			}
			timeout.Reset(interframeTimeout)

		case <-timeout.C:
			if len(m.buf) != 0 && !bufok && nto < 10 {
				nto++
				timeout.Reset(interframeTimeout)
				continue
			}
			break readLoop
		case err = <-m.IntrC:
			break readLoop
		}
	}
	m.req <- nil

	buf = m.buf
	if err == nil && len(buf) == 0 {
		err = modbus.ErrTimeout
	}
	return
}

type readResult struct {
	data []byte
	err  error
}

func (m *ReadMgr) handle(read ReadFunc, exitC chan<- int) {
	var termErr error
	var dest []byte
	var errC chan<- error

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
		case errC <- termErr:
			close(errC)
			break loop
		case dest = <-m.req:
		case r, dataOk := <-data:
			if dest != nil {
				if r.err == nil {
					dest = append(dest, r.data...)
				}
			} else if m.Forward != nil {
				m.Forward.Write(r.data)
			}
			if dataOk {
				data <- readResult{}
			} else {
				data = nil
			}
			if dest != nil {
				select {
				case m.done <- readResult{dest, r.err}:
					if r.err != nil {
						break loop
					}
				case b := <-m.req:
					if m.Forward != nil {
						m.Forward.Write(dest)
					}
					dest = b
				}
			}
			if r.err != nil && errC == nil {
				termErr = r.err
				errC = m.errC
			}
		}
	}
	close(m.done)
}
