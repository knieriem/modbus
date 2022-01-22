package debug

import (
	"fmt"

	"github.com/knieriem/modbus"
)

func FormatMsg(msgDir string, adu modbus.ADU, err error, ncName string) string {
	s := ""
	if msgDir != "" {
		s += msgDir + " "
	}
	s += ncName
	n := len(adu.Bytes)
	if !adu.Valid() {
		if n == 0 {
			s += " [0]"
		} else {
			s += fmt.Sprintf(" [%d] % x", n, adu.Bytes)
		}
	} else {
		header := adu.Bytes[:adu.PDUStart]
		end := n + adu.PDUEnd
		pdu := adu.Bytes[adu.PDUStart:end]
		s += fmt.Sprintf(" [%d] (% x) % x", len(pdu), header, pdu)
		if adu.PDUEnd < 0 {
			s += fmt.Sprintf(" (% x)", adu.Bytes[end:])
		}
	}
	if err != nil {
		s += " error: " + err.Error()
	}
	return s
}
