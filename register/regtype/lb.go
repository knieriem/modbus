package regtype

import (
	"encoding/binary"
)

type littleEndianBytesSwapped struct{}

func (littleEndianBytesSwapped) String() string { return "LittleEndianBytesSwapped" }

func swap16(b []byte) {
	b[0], b[1] = b[1], b[0]
}

func (littleEndianBytesSwapped) Uint16(b []byte) uint16 {
	return binary.BigEndian.Uint16(b)
}

func (littleEndianBytesSwapped) PutUint16(b []byte, v uint16) {
	binary.BigEndian.PutUint16(b, v)
}

func (littleEndianBytesSwapped) Uint32(b []byte) uint32 {
	swap16(b[0:])
	swap16(b[2:])
	return binary.LittleEndian.Uint32(b)
}

func (littleEndianBytesSwapped) PutUint32(b []byte, v uint32) {
	binary.LittleEndian.PutUint32(b, v)
	swap16(b[0:])
	swap16(b[2:])
}

func (littleEndianBytesSwapped) Uint64(b []byte) uint64 {
	swap16(b[0:])
	swap16(b[2:])
	swap16(b[4:])
	swap16(b[6:])
	return binary.LittleEndian.Uint64(b)
}

func (littleEndianBytesSwapped) PutUint64(b []byte, v uint64) {
	binary.LittleEndian.PutUint64(b, v)
	swap16(b[0:])
	swap16(b[2:])
	swap16(b[4:])
	swap16(b[6:])
}
