package register

import (
	"bytes"
	"math"
	"reflect"

	"github.com/knieriem/modbus"
)

type Float32Big float32

func (f Float32Big) Value() float32 {
	return float32(f)
}

func (f Float32Big) Set(v float32) {
	f = Float32Big(v)
}

func decodeFloat32(f [4]byte) float32 {
	return math.Float32frombits(modbus.ByteOrder.Uint32(f[:]))
}

func encodeFloat32(f []byte, v float32) {
	modbus.ByteOrder.PutUint32(f, math.Float32bits(v))
}

type Float32BigBS [4]byte

func (f Float32BigBS) Value() float32 {
	f[0], f[1], f[2], f[3] = f[1], f[0], f[3], f[2]
	return decodeFloat32(f)
}

func (f Float32BigBS) Set(v float32) {
	encodeFloat32(f[:], v)
	f[1], f[0], f[3], f[2] = f[0], f[1], f[2], f[3]
	return
}

type Float32LittleBS [4]byte

type Float32Little [4]byte

func (f Float32LittleBS) Value() float32 {
	f[0], f[1], f[2], f[3] = f[2], f[3], f[0], f[1]
	return decodeFloat32(f)
}

func (f Float32LittleBS) Set(v float32) {
	encodeFloat32(f[:], v)
	f[2], f[3], f[0], f[1] = f[0], f[1], f[2], f[3]
	return
}

func (f Float32Little) Value() float32 {
	f[0], f[1], f[2], f[3] = f[3], f[2], f[1], f[0]
	return decodeFloat32(f)
}

func (f Float32Little) Set(v float32) {
	encodeFloat32(f[:], v)
	f[3], f[2], f[1], f[0] = f[0], f[1], f[2], f[3]
	return
}

type Uint32LittleBS [2]uint16

func (v Uint32LittleBS) Value() uint32 {
	return uint32(v[1])<<16 | uint32(v[0])
}

type Int32LittleBS [2]uint16

func (v Int32LittleBS) Value() int32 {
	return int32(Uint32LittleBS(v).Value())
}

type Int32Big int32

func (v Int32Big) Value() int32 {
	return int32(v)
}

type Uint32Big uint32

func (v Uint32Big) Value() uint32 {
	return uint32(v)
}

type PackedBytesBig [2]byte

func (p PackedBytesBig) decode() (b0, b1 byte) {
	b0, b1 = p[0], p[1]
	return
}

func (p *PackedBytesBig) encode(b0, b1 byte) {
	p[0], p[1] = b0, b1
	return
}

type PackedBytesLittle [2]byte

func (p PackedBytesLittle) decode() (b0, b1 byte) {
	b0, b1 = p[1], p[0]
	return
}

func (p *PackedBytesLittle) encode(b0, b1 byte) {
	p[0], p[1] = b1, b0
	return
}

type packedBytesEncoder interface {
	encode(b0, b1 byte)
}
type packedBytesDecoder interface {
	decode() (b0, b1 byte)
}

type PackedBytesBuf interface{}

func packedBytesBufValue(src PackedBytesBuf) (n int, v reflect.Value) {
	v = reflect.ValueOf(src)
	k := v.Kind()
	if k == reflect.Ptr {
		v = v.Elem()
		k = v.Kind()
	}
	if k != reflect.Slice && k != reflect.Array {
		panic("not a slice/array")
	}
	n = v.Len()
	return
}

func DecodeString(src PackedBytesBuf, filters ...func([]byte) []byte) string {
	n, v := packedBytesBufValue(src)
	buf := make([]byte, 0, n*2)
	for i := 0; i < n; i++ {
		d := v.Index(i).Interface().(packedBytesDecoder)
		b0, b1 := d.decode()
		buf = append(buf, b0, b1)
	}
	for _, f := range filters {
		buf = f(buf)
	}
	return string(buf)
}

func StopAtZero(buf []byte) []byte {
	if i := bytes.IndexAny(buf, "\x00\xff"); i != -1 {
		buf = buf[:i]
	}
	return buf
}

func TrimLeftSpace(buf []byte) []byte {
	return bytes.TrimLeft(buf, " \x00\xff")
}

func TrimRightSpace(buf []byte) []byte {
	return bytes.TrimRight(buf, " \x00\xff")
}

func EncodeString(dest PackedBytesBuf, s string) {
	n, v := packedBytesBufValue(dest)
	for i := 0; i < n; i++ {
		var b0, b1 byte
		switch len(s) {
		default:
			b0, b1 = s[0], s[1]
			s = s[2:]
		case 1:
			b0 = s[0]
			s = s[1:]
		case 0:
		}
		d := v.Index(i).Addr().Interface().(packedBytesEncoder)
		d.encode(b0, b1)
	}
}
