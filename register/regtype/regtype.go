package regtype

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/knieriem/modbus"
	"github.com/knieriem/modbus/register"

	"github.com/h2so5/half"
)

type ModifierFunc func(BaseValue) BaseValue

type BaseValue interface {
	baseValue
}

var modMap map[string]ModifierFunc

func RegisterModifier(name string, f ModifierFunc) {
	if modMap == nil {
		modMap = make(map[string]ModifierFunc, 1)
	}
	modMap[name] = f
}

type decoder func(interface{})

type def struct {
	makeSlice func(n int) interface{}
	parse     func(string) (baseValue, error)
	size      int
	fmt       string
}

type baseValue interface {
	Format() string
	Value() interface{}
}

type Value struct {
	baseValue
	byteOrder binary.ByteOrder
}

func (v Value) String() string {
	return v.Format()
}

func (v Value) Err() error {
	return inbandErr(v.baseValue)
}

var types = map[string]*def{
	"f": {
		makeSlice: makeFloat32,
		parse:     newFloat32,
		size:      2,
	},
	"f16": {
		makeSlice: makeFloat16,
		parse:     newFloat16,
		size:      1,
	},
	"f32": {
		makeSlice: makeFloat32,
		parse:     newFloat32,
		size:      2,
	},
	"f64": {
		makeSlice: makeFloat64,
		parse:     newFloat64,
		size:      4,
	},
	"u": {
		makeSlice: makeUint16,
		parse:     newUint16,
		size:      1,
	},
	"u32": {
		makeSlice: makeUint32,
		parse:     newUint32,
		size:      2,
	},
	"u64": {
		makeSlice: makeUint64,
		parse:     newUint64,
		size:      4,
	},
	"i": {
		makeSlice: makeInt16,
		parse:     newInt16,
		size:      1,
	},
	"i32": {
		makeSlice: makeInt32,
		parse:     newInt32,
		size:      2,
	},
	"i64": {
		makeSlice: makeInt64,
		parse:     newInt64,
		size:      4,
	},
	"x": {
		makeSlice: makeUint16,
		fmt:       "%x",
		size:      1,
	},
	"x32": {
		makeSlice: makeUint32,
		fmt:       "%x",
		size:      2,
	},
	"c": {
		makeSlice: makeString,
		parse:     newString,
		size:      1,
	},
	"cs": {
		makeSlice: makeStringBS,
		parse:     newStringBS,
		size:      1,
	},
	"_": {
		makeSlice: makeIgnored,
		parse:     newIgnored,
		size:      1,
	},
}

type Ignored uint16

func (u Ignored) Format() string {
	return "_"
}

func (u Ignored) Value() interface{} {
	return u
}

func makeIgnored(n int) interface{} {
	return make([]Ignored, n)
}

func newIgnored(s string) (v baseValue, err error) {
	return Ignored(0), nil
}

func formatUint(u uint64) string {
	return strconv.FormatUint(u, 10)
}

type Uint16 uint16

func (u Uint16) Format() string {
	return formatUint(uint64(u))
}

func (u Uint16) Value() interface{} {
	return u
}

func (u Uint16) float() float64 {
	return float64(u)
}

func makeUint16(n int) interface{} {
	return make([]Uint16, n)
}

func newUint16(s string) (v baseValue, err error) {
	n, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return
	}
	v = Uint16(n)
	return
}

type Uint32 uint32

func (u Uint32) Format() string {
	return strconv.FormatUint(uint64(u), 10)
}

func (u Uint32) Value() interface{} {
	return u
}

func (u Uint32) float() float64 {
	return float64(u)
}

func makeUint32(n int) interface{} {
	return make([]Uint32, n)
}

func newUint32(s string) (v baseValue, err error) {
	n, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return
	}
	v = Uint32(n)
	return
}

type Uint64 uint64

func (u Uint64) Format() string {
	return strconv.FormatUint(uint64(u), 10)
}

func (u Uint64) Value() interface{} {
	return u
}

func (u Uint64) float() float64 {
	return float64(u)
}

func makeUint64(n int) interface{} {
	return make([]Uint64, n)
}

func newUint64(s string) (v baseValue, err error) {
	n, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return
	}
	v = Uint64(n)
	return
}

type Int16 int16

func (i Int16) Format() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int16) Value() interface{} {
	return i
}

func (i Int16) float() float64 {
	return float64(i)
}

func makeInt16(n int) interface{} {
	return make([]Int16, n)
}

func newInt16(s string) (v baseValue, err error) {
	n, err := strconv.ParseInt(s, 0, 16)
	if err != nil {
		return
	}
	v = Int16(n)
	return
}

type Int32 int32

func (i Int32) Format() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int32) Value() interface{} {
	return i
}

func (i Int32) float() float64 {
	return float64(i)
}

func makeInt32(n int) interface{} {
	return make([]Int32, n)
}

func newInt32(s string) (v baseValue, err error) {
	n, err := strconv.ParseInt(s, 0, 32)
	if err != nil {
		return
	}
	v = Int32(n)
	return
}

type Int64 int64

func (i Int64) Format() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int64) Value() interface{} {
	return i
}

func (i Int64) float() float64 {
	return float64(i)
}

func makeInt64(n int) interface{} {
	return make([]Int64, n)
}

func newInt64(s string) (v baseValue, err error) {
	n, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return
	}
	v = Int64(n)
	return
}

type Float16 uint16

func (f Float16) Format() string {
	return strconv.FormatFloat(float64(f.float32()), 'g', -1, 32)
}

func (f Float16) Value() interface{} {
	return f.float32()
}

func (f Float16) float32() float32 {
	return half.Float16(f).Float32()
}

func makeFloat16(n int) interface{} {
	return make([]Float16, n)
}

func newFloat16(s string) (v baseValue, err error) {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return
	}
	return Float16(half.NewFloat16(float32(f))), nil
}

type Float32 float32

func (f Float32) Format() string {
	return strconv.FormatFloat(float64(f), 'g', -1, 32)
}

func (f Float32) Value() interface{} {
	return f
}

func makeFloat32(n int) interface{} {
	return make([]Float32, n)
}

func newFloat32(s string) (v baseValue, err error) {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return
	}
	v = Float32(f)
	return
}

type Float64 float64

func (f Float64) Format() string {
	return strconv.FormatFloat(float64(f), 'g', -1, 64)
}

func (f Float64) Value() interface{} {
	return f
}

func makeFloat64(n int) interface{} {
	return make([]Float64, n)
}

func newFloat64(s string) (v baseValue, err error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return
	}
	v = Float64(f)
	return
}

type String []register.PackedBytesBig

func (s String) Format() string {
	return strconv.Quote(register.DecodeString(s, register.TrimRightSpace))
}

func (s String) Value() interface{} {
	return s
}

func makeString(n int) interface{} {
	return String(make([]register.PackedBytesBig, n))
}

func newstr(s string, mk func(n int) interface{}) (v baseValue, err error) {
	v = mk((len(s) + 1) / 2).(baseValue)
	register.EncodeString(v, s)
	return
}

func newString(s string) (baseValue, error) {
	return newstr(s, makeString)
}

type StringBS []register.PackedBytesLittle

func (s StringBS) Format() string {
	return strconv.Quote(register.DecodeString(s, register.TrimRightSpace))
}

func (s StringBS) Value() interface{} {
	return s
}

func makeStringBS(n int) interface{} {
	return StringBS(make([]register.PackedBytesLittle, n))
}

func newStringBS(s string) (v baseValue, err error) {
	return newstr(s, makeStringBS)
}

type procValue struct {
	baseValue
	opts string
}

func (v *procValue) Value() interface{} {
	var s string
	quote := false
	switch b := v.baseValue.(type) {
	case StringBS, String:
		filters := make([]func([]byte) []byte, 0, len(v.opts))
		for _, c := range v.opts {
			switch c {
			case 0:
				return errors.New(",!(procOpt missing)")
			default:
				return fmt.Errorf(",!(procOpt=%q)", c)
			case '<':
				filters = append(filters, register.TrimRightSpace)
			case '>':
				filters = append(filters, register.TrimLeftSpace)
			case '0':
				filters = append(filters, register.StopAtZero)
			case 'o':
				filters = append(filters, func(b []byte) []byte {
					if n := len(b); n > 1 {
						b = b[:n-1]
					}
					return b
				})
			case 'q':
				quote = true
			}
		}
		s = register.DecodeString(b, filters...)
	default:
		return v.baseValue.Value()
	}
	if quote {
		s = strconv.Quote(s)
	}
	return s
}

func (v *procValue) Format() string {
	switch v.baseValue.(type) {
	case StringBS, String:
		return v.Value().(string)
	}
	return v.baseValue.Format()
}

type fmtValue struct {
	baseValue
	fmt string
}

func (v *fmtValue) Format() string {
	return fmt.Sprintf(v.fmt, v.Value())
}

type divValue struct {
	baseValue
	div  uint
	prec int
}

type floater interface {
	float() float64
}

func (v *divValue) Value() interface{} {
	if f, ok := v.baseValue.Value().(floater); ok {
		return f.float() / float64(v.div)
	}
	return v.baseValue.Value()
}

func (div *divValue) Format() string {
	v := div.Value()
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', div.prec, 64)
	}
	return "%!not a float"
}

func parseValueSpec(dest []Value, s string) (vlist []Value, nRegs int, err error) {
	typeSpec := ""

	if i := strings.Index(s, "("); i != -1 {
		if !strings.HasSuffix(s, ")") {
			err = errors.New("missing ')'")
			return
		}
		typeSpec = s[:i]
		s = s[i+1 : len(s)-1]
	} else {
		switch {
		case strings.HasPrefix(s, "-"):
			typeSpec = "i"
		case strings.Index(s, ".") != -1:
			typeSpec = "f"
		default:
			typeSpec = "u"
		}
	}
	ts, err := scanTypeSpec(typeSpec)
	if err != nil {
		return
	}
	d, ok := types[ts.name]
	if !ok {
		err = errors.New("unknown type")
		return
	}

	count := ts.n
	var args []string
	if strings.HasPrefix(s, `"`) {
		s, err = strconv.Unquote(s)
		if err != nil {
			return
		}
		n := (len(s) + 1) / 2
		if count == 0 {
			if n == 0 {
				count = 1
			} else {
				count = n
			}
		}
		for ; n < count; n++ {
			s += "\x00\x00"
		}
		args = []string{s}
	} else {
		args = strings.Split(strings.TrimSpace(s), " ")
		if count == 0 {
			count = len(args)
		} else if count != len(args) {
			err = errors.New("number of values doesn't match the specified count")
			return
		}
	}
	vlist = dest
	for _, f := range args {
		v, err1 := d.parse(f)
		if err1 != nil {
			err = err1
			return
		}
		vlist = append(vlist, Value{baseValue: v, byteOrder: ts.byteOrder})
	}
	nRegs = count * d.size
	return
}

type TypeSpec struct {
	*def
	byteOrder binary.ByteOrder
	fmt       string
	n         int
	div       uint
	divDigits int
	name      string
	mf        ModifierFunc
	procOpts  string
}

func (ts *TypeSpec) NReg() int {
	return ts.n * ts.def.size
}

func ParseTypeSpec(s string) (ts *TypeSpec, err error) {
	ts, err = scanTypeSpec(s)
	if err != nil {
		return nil, err
	}
	if ts.n == 0 {
		ts.n = 1
	}
	if ts.name == "" {
		ts.name = "u"
	}
	d, ok := types[ts.name]
	if !ok {
		return nil, errors.New("unknown type")
	}
	if ts.fmt == "" {
		ts.fmt = d.fmt
	}
	ts.def = d
	return ts, nil
}

func scanTypeSpec(s string) (*TypeSpec, error) {
	count := s
	typeName := ""
	for i, r := range s {
		if r >= '1' && r <= '9' {
			continue
		}
		if i != 0 && r == '0' {
			continue
		}
		count = s[:i]
		typeName = s[i:]
		if r == '*' && len(typeName) != 1 {
			typeName = typeName[1:]
		}
		break
	}
	ts := new(TypeSpec)
	if count != "" {
		n64, err := strconv.ParseUint(count, 10, 8)
		if err != nil {
			return nil, err
		}
		ts.n = int(n64)
	}
	if i := strings.LastIndexByte(typeName, '/'); i != -1 {
		divstr := typeName[i:]
		typeName = typeName[:i]
		if i := strings.LastIndexByte(divstr, '%'); i != -1 {
			ts.fmt = divstr[i:]
			divstr = divstr[:i]
		}
		u, err := strconv.ParseUint(divstr[1:], 10, 32)
		if err != nil {
			return nil, err
		}
		ts.div = uint(u)
		switch {
		default:
			ts.divDigits = 0
		case ts.div > 1e5:
			ts.divDigits = 6
		case ts.div > 1e4:
			ts.divDigits = 5
		case ts.div > 1e3:
			ts.divDigits = 4
		case ts.div > 100:
			ts.divDigits = 3
		case ts.div > 10:
			ts.divDigits = 2
		case ts.div > 1:
			ts.divDigits = 1
		}
	} else if i := strings.LastIndex(typeName, "%"); i != -1 {
		ts.fmt = typeName[i:]
		typeName = typeName[:i]
	}
	if i := strings.IndexByte(typeName, '.'); i != -1 {
		mod := typeName[i+1:]
		mf, ok := modMap[mod]
		if !ok {
			return ts, errors.New("unknown modifier: " + strconv.Quote(mod))
		}
		ts.mf = mf
		typeName = typeName[:i]
	}
	if i := strings.IndexByte(typeName, ','); i != -1 {
		o := typeName[i+1:]
		if o == "" {
			o = "\x00"
		}
		ts.procOpts = o
		typeName = typeName[:i]
	}
	if n := len(typeName); n > 4 {
	testByteOrderSuffix:
		switch typeName[n-3] {
		case '6', '2', '4':
			switch typeName[n-2:] {
			default:
				break testByteOrderSuffix
			case "le":
				ts.byteOrder = binary.LittleEndian
			case "lb":
				ts.byteOrder = littleEndianBytesSwapped{}
			}
			typeName = typeName[:n-2]
			if strings.HasSuffix(typeName, "16") {
				typeName = typeName[:n-4]
			}
		}
	}
	ts.name = typeName
	return ts, nil
}

func ParseValues(values []string) (vlist []Value, nRegs int, err error) {
	var bracedExpr string

	for _, s := range values {
		var n int
		if bracedExpr == "" {
			if strings.Index(s, "(") != -1 && strings.Index(s, ")") == -1 {
				bracedExpr = s
				continue
			}
		} else {
			if strings.Index(s, ")") == -1 {
				bracedExpr += " " + s
				continue
			}
			s = bracedExpr + " " + s
			bracedExpr = ""
		}

		vlist, n, err = parseValueSpec(vlist, s)
		if err != nil {
			return
		}
		nRegs += n
	}
	if bracedExpr != "" {
		err = errors.New("missing ')'")
	}
	return
}

func ParseSpecs(specs []string) (list []*TypeSpec, nBytes int, err error) {
	for _, s := range specs {
		ts, err := ParseTypeSpec(s)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, ts)
		nBytes += ts.NReg() * 2
	}
	return list, nBytes, err
}

type EncodingOption func(*encOptions)

type encOptions struct {
	byteOrder binary.ByteOrder
}

// LittleEndianHack reverses the byte order for all types.
// For some Modbus device implementations, the 16-bit register byte order,
// contrary to the Modbus specification, is assumed to be little endian.
// With this hack communicating with these devices remains possible
// without providing special little-endian types. Use with caution.
func LittleEndianHack() EncodingOption {
	return func(o *encOptions) {
		o.byteOrder = binary.LittleEndian
	}
}

func Encode(b []byte, vlist []Value, opts ...EncodingOption) (err error) {
	e := setupEncOptions(opts)
	w := bytes.NewBuffer(b[:0])
	for _, v := range vlist {
		bo := e.byteOrder
		if v.byteOrder != nil {
			bo = v.byteOrder
		}
		err = binary.Write(w, bo, v.baseValue)
		if err != nil {
			return
		}
	}
	return
}

func Decode(b []byte, specs []*TypeSpec, opts ...EncodingOption) []Value {
	e := setupEncOptions(opts)

	r := bytes.NewReader(b)

	/* pre-allocate vlist */
	numVal := 0
	for _, ts := range specs {
		numVal += ts.n
	}
	vlist := make([]Value, 0, numVal)

	for _, ts := range specs {
		sl := ts.makeSlice(ts.n)
		bo := e.byteOrder
		if ts.byteOrder != nil {
			bo = ts.byteOrder
		}
		err := binary.Read(r, bo, sl)
		if err != nil {
			return nil
		}
		if bv, ok := sl.(baseValue); ok {
			if inbandErr(bv) == nil {
				if ts.procOpts != "" {
					bv = &procValue{opts: ts.procOpts, baseValue: bv}
				}
			}
			vlist = append(vlist, Value{baseValue: bv})
			continue
		}
		v := reflect.ValueOf(sl)
		for i := 0; i < ts.n; i++ {
			val := v.Index(i).Interface().(baseValue)
			if mf := ts.mf; mf != nil {
				val = ts.mf(val)
			}
			if inbandErr(val) == nil {
				if ts.div != 0 {
					val = &divValue{div: ts.div, baseValue: val, prec: ts.divDigits}
				}
				if ts.fmt != "" {
					val = &fmtValue{fmt: ts.fmt, baseValue: val}
				}
			}
			vlist = append(vlist, Value{baseValue: val})
		}
	}
	return vlist
}

func setupEncOptions(opts []EncodingOption) *encOptions {
	var e encOptions

	e.byteOrder = modbus.ByteOrder
	for _, o := range opts {
		o(&e)
	}
	return &e
}

type inbandError interface {
	Err() error
}

func inbandErr(v BaseValue) error {
	if e, ok := v.(inbandError); ok {
		return e.Err()
	}
	return nil
}

func AttachErr(v BaseValue, err error) BaseValue {
	return &baseValueWithErr{
		baseValue: v,
		error:     err,
	}
}

type baseValueWithErr struct {
	baseValue
	error
}

func (ev *baseValueWithErr) Err() error {
	return ev.error
}

var ErrValNone = ErrStr("noValue")
var ErrValUnspecified = ErrStr("")

type ErrStr string

func (s ErrStr) String() string {
	if s == "" {
		return "err"
	}
	return "err." + string(s)
}

func (s ErrStr) Error() string {
	return s.String()
}
