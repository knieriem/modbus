package netconn

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/knieriem/text/tidata"
	"modbus"
)

var protos = make(map[string]*Proto, 4)
var defaultProto *Proto

func SetDefaultProto(name string) {
	defaultProto = protos[name]
}

func RegisterProtocol(proto *Proto) {
	protos[proto.Name] = proto
}

func (c *Conf) proto() (p *Proto, err error) {
	p, ok := protos[c.Proto]
	if !ok {
		err = errors.New("invalid proto")
	}
	return
}

const (
	FieldAddr = 1 << iota
	FieldDev
	FieldOpt
	FieldTxID
	FieldRxID
	endField          = 1 << iota
	FieldMask         = endField - 1
	DevFields         = FieldDev | FieldOpt
	CanIDFields       = FieldRxID | FieldTxID
	FieldsOverridable = CanIDFields
)

var fieldNameMap = map[int]string{
	FieldAddr: "Addr",
	FieldDev:  "Device",
	FieldOpt:  "Options",
	FieldTxID: "Txid",
	FieldRxID: "Rxid",
}
var overridableFieldsMap = map[string]int{
	"txid": FieldTxID,
	"rxid": FieldRxID,
}

type Proto struct {
	Name           string
	Dial           func(*Conf) (*Conn, error)
	RequiredFields int
	OptionalFields int
	InterfaceGroup *InterfaceGroup
}

func (p *Proto) UnexpectedFields() int {
	return ^(p.RequiredFields | p.OptionalFields) & FieldMask
}

func (p *Proto) fieldFlags() int {
	return p.RequiredFields | p.OptionalFields
}

type Conn struct {
	modbus.NetConn
	io.Closer
	Addr       string
	Device     string
	DeviceInfo string
	ExitC      <-chan int
}

type Conf struct {
	tidataInfo

	Proto   string
	Name    string
	Addr    IPAddr
	Device  string
	Options []string
	Txid    CanID
	Rxid    CanID

	Default bool
}

type tidataInfo struct {
	SrcLineNum int
	TidataSeen map[string]bool
}

func (inf *tidataInfo) Seen(field string) bool {
	return inf.TidataSeen[field]
}

func (c *Conf) Dial() (conn *Conn, err error) {
	p, err := c.proto()
	if err != nil {
		return
	}
	return p.Dial(c)
}

func (c *Conf) MakeAddr(name string, addOptions bool) (addr string) {
	addr = c.Name
	if addr == "" {
		addr = c.Proto
	}
	addr += ":" + name
	if addOptions && len(c.Options) != 0 {
		addr += "," + strings.Join(c.Options, ",")
	}
	return
}

func (c *Conf) SupportsOptions() bool {
	p, ok := protos[c.Proto]
	return ok && (p.fieldFlags()&FieldOpt != 0)
}

func (c *Conf) InterfaceName() string {
	p, ok := protos[c.Proto]
	if !ok {
		return ""
	}
	flags := p.fieldFlags()
	if flags&FieldDev != 0 {
		return c.Device
	}
	if flags&FieldAddr != 0 {
		return string(c.Addr)
	}
	return ""
}

func (c *Conf) SetInterfaceName(name string) {
	p, ok := protos[c.Proto]
	if !ok {
		return
	}
	flags := p.fieldFlags()
	if flags&FieldDev != 0 {
		c.Device = name
	}
	if flags&FieldAddr != 0 {
		c.Addr = IPAddr(name)
	}
}

func (c *Conf) DefaultInterfaceName() string {
	p, ok := protos[c.Proto]
	if !ok {
		return ""
	}
	list := p.InterfaceGroup.Interfaces()
	if len(list) == 0 {
		return ""
	}
	return list[0].Name
}

func (c *Conf) InterfaceType() string {
	p, ok := protos[c.Proto]
	if !ok {
		return ""
	}
	return p.InterfaceGroup.Type
}

func (c *Conf) Interfaces() []Interface {
	p, ok := protos[c.Proto]
	if ok {
		return p.InterfaceGroup.Interfaces()
	}
	return nil
}

func (c *Conf) Postprocess() (err error) {
	if c.Proto == "" {
		err = errors.New("missing value for protocol")
		return
	}
	p, ok := protos[c.Proto]
	if !ok {
		// unsupported, ignore for now
		return
	}
	unexpected := p.UnexpectedFields()
	for f := 1; f < endField; f <<= 1 {
		if p.RequiredFields&f != 0 {
			field := fieldNameMap[f]
			if !c.Seen(field) {
				err = errors.New("required field missing: " + field)
			}
		}
		if unexpected&f != 0 {
			field := fieldNameMap[f]
			if c.Seen(field) {
				err = errors.New("unexpected field: " + field)
			}
		}
	}
	if strings.HasSuffix(c.Name, "*") {
		c.Default = true
		c.Name = c.Name[:len(c.Name)-1]
	}
	return
}

type IPAddr string

func (a *IPAddr) UnmarshalTidata(el tidata.Elem) (err error) {
	*a = IPAddr(el.Value())
	_, err = a.Complete("9999")
	return
}

func (a IPAddr) Complete(defaultPort string) (hostport string, err error) {
	addr := string(a)
	hostport = addr
	switch {
	case strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]"):
		fallthrough
	case strings.LastIndex(addr, ":") == -1:
		hostport = addr + ":" + defaultPort
	}
	_, _, err = net.SplitHostPort(hostport)
	return
}

type CanID struct {
	ID       uint32
	Extframe bool
}

func (can *CanID) String() string {
	s := strconv.FormatUint(uint64(can.ID), 16)
	s = strings.ToUpper(s)
	if can.Extframe {
		s = "E/" + s
	}
	return s
}

func (can *CanID) UnmarshalTidata(el tidata.Elem) (err error) {
	id := el.Value()
	if id == "" {
		err = errors.New("missing value")
		return
	}
	x := strings.HasPrefix(id, "E/")
	if x {
		id = id[2:]
	}
	u, err := strconv.ParseUint(id, 0, 32)
	if err == nil {
		if x {
			if u >= 1<<29 {
				err = errors.New("value does not fit into 29 bit")
				return
			}
		} else if u >= 1<<11 {
			err = errors.New("value does not fit into 11 bit")
		}
		can.ID = uint32(u)
		can.Extframe = x
	}
	return
}

type ConfList []*Conf

func (list ConfList) Names() []string {
	names := make([]string, len(list))
	for i, c := range list {
		name := c.Name
		if name == "" {
			name = c.Proto
		}
		names[i] = name
	}
	return names
}

func (list ConfList) Postprocess() (err error) {
	usedProtos := make(map[string]bool, len(protos))
	usedNames := make(map[string]bool, len(protos))
	foundDefault := false

	for _, c := range list {
		usedProtos[c.Proto] = true
		if name := c.Name; name != "" {
			if usedNames[name] {
				err = errors.New("name used more than once: " + name)
				return
			}
			usedNames[name] = true
		}
		if c.Default {
			if foundDefault {
				err = errors.New("more than one marked as default")
				return
			}
			foundDefault = true
		}
	}
	for _, c := range list {
		if usedProtos[c.Name] {
			err = errors.New("proto name used as netconn name: " + c.Name)
			return
		}
	}
	return
}

func (list ConfList) Default() (index int) {
	for i, c := range list {
		if c.Default {
			index = i
			break
		}
	}
	return
}

type nameSpec struct {
	name    string
	options []string
}

func splitSpec(connSpec string) (ns []nameSpec) {
	for _, f := range strings.SplitN(connSpec, ":", 2) {
		fs := strings.Split(f, ",")
		ns = append(ns, nameSpec{name: fs[0], options: fs[1:]})
	}
	return
}

func (c *Conf) derive(f []nameSpec) (dc *Conf, err error) {
	var m Conf

	m = *c
	p, err := c.proto()
	if err != nil {
		return
	}

	flags := p.fieldFlags()
	if len(f) == 2 {
		if s := f[1].name; s != "" {
			if flags&FieldDev != 0 {
				m.Device = s
				dc = &m
			}
			if flags&FieldAddr != 0 {
				m.Addr = IPAddr(s)
				dc = &m
			}
		}
		if flags&FieldOpt != 0 {
			if s := f[1].options; len(s) != 0 {
				m.Options = s
				dc = &m
			}
		}
	}

	flags &= FieldsOverridable
optLoop:
	for i, o := range f[0].options {
		of := strings.SplitN(o, "=", 2)
		switch len(of) {
		case 1:
			m.Options = f[0].options[i:]
			dc = &m
			break optLoop
		case 2:
			oflag := overridableFieldsMap[of[0]]
			if oflag == 0 {
				if len(f) == 2 {
					err = errors.New("unknown netconn option: " + of[0])
					return
				}
				m.Options = f[0].options[i:]
				dc = &m
				break optLoop
			} else if oflag&flags == 0 {
				if len(f) == 2 {
					err = errors.New("option not allowed here: " + of[0])
					return
				}
				m.Options = f[0].options[i:]
				dc = &m
				break optLoop
			} else {
				fmt.Println("FLAG", f[0], f[1])
				dc = &m
			}
		}
	}
	return
}

func (list ConfList) Match(connSpec string) (index int, mod *Conf, err error) {
	if connSpec == "" {
		index = list.Default()
		return
	}
retry:
	f := splitSpec(connSpec)
	if net := f[0].name; net != "" {
		// name present, select matching entry
		for i, c := range list {
			if c.Name == net || c.Proto == net {
				index = i
				mod, err = c.derive(f)
				return
			}
		}
		if len(f) == 2 {
			err = errors.New("no matching network connection")
			return
		}
		if p := defaultProto; p != nil {
			connSpec = p.Name + ":" + connSpec
		}
		goto retry
	}
	index = list.Default()
	mod, err = list[index].derive(f)
	return
}

func (list ConfList) Dial(connSpec string) (conn *Conn, err error) {
	index, cf, err := list.Match(connSpec)
	if err != nil {
		return
	}
	if cf == nil {
		cf = list[index]
	}
	return cf.Dial()
}
