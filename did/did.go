// Package did implements the Modbus Read Device Identification function
package did

import (
	"bytes"
	"encoding/binary"
	"io"
	"unicode/utf8"

	"github.com/knieriem/modbus"
	"github.com/knieriem/modbus/mei"
)

type Error string

func (e Error) Error() string {
	return "did: " + string(e)
}

type Category byte

const (
	Basic Category = 1 + iota
	Regular
	Extended
	Single
)

type Object struct {
	ID
	Data []byte
}

// IsString reports whether the object data consists of valid UTF8 characters.
func (o *Object) IsString() bool {
	return utf8.ValidString(o.String())
}

func (o *Object) String() string {
	return string(o.Data)
}

type ID byte

func (id ID) Name() string {
	if int(id) < len(names) {
		return names[id]
	}
	return ""
}

const (
	VendorName ID = iota
	ProductCode
	MajorMinorRevision
	VendorURL
	ProductName
	ModelName
	UserApplName
)

var names = []string{
	VendorName:         "VendorName",
	ProductCode:        "ProductCode",
	MajorMinorRevision: "MajorMinorRevision",
	VendorURL:          "VendorURL",
	ProductName:        "ProductName",
	ModelName:          "ModelName",
	UserApplName:       "UserApplicationName",
}

type Reader struct {
	tp *mei.Transport
}

func NewReader(d modbus.Device) *Reader {
	r := new(Reader)
	r.tp = mei.NewTransport(d, 14)
	return r
}

type respHdr struct {
	ReadDIDCode byte
	Conformity  byte
	MoreFollows byte
	NextObjID   byte
	NObj        byte
}

func (r *Reader) ReadObject(id ID, reqOpts ...modbus.ReqOption) (o Object, err error) {
	list, err := r.Read(Single, id, reqOpts...)
	if err != nil {
		return
	}
	o = list[0]
	return
}

func (r *Reader) Read(cat Category, startID ID, reqOpts ...modbus.ReqOption) (list []Object, err error) {
	forceID := false
more:
	req := []byte{byte(cat), byte(startID)}
	vs := &modbus.VariableRespLenSpec{
		NumItemsIndex: 6,
		ItemLenIndex:  1,
	}
	opts := append(reqOpts, modbus.VariableRespLen(vs))
	resp, err := r.tp.Request(req, opts...)
	if err != nil {
		return
	}

	var h respHdr
	br := bytes.NewBuffer(resp)
	err = binary.Read(br, binary.BigEndian, &h)
	if err != nil {
		if err == io.EOF {
			err = Error("invalid msg len")
		}
		return
	}

	data := br.Bytes()
	if h.NObj == 0 {
		if len(data) != 0 {
			err = Error("invalid number of objects")
		}
		return
	}
	if cat == Single {
		if h.MoreFollows != 0 || h.NextObjID != 0 || h.NObj != 1 {
			err = Error("invalid header values in a response to an individual access")
			return
		}
	}

	var o Object
	for i := 0; i < int(h.NObj); i++ {
		data, err = parseObject(&o, data)
		if err != nil {
			return
		}
		if forceID && i == 0 {
			if o.ID != startID {
				err = Error("start ID of continuation does not match")
				return
			}
		}
		list = append(list, o)
	}
	if len(data) != 0 {
		err = Error("unexpected trailing bytes")
		return
	}
	if h.MoreFollows != 0 {
		forceID = true
		startID = ID(h.NextObjID)
		goto more
	}
	return
}

func parseObject(o *Object, data []byte) (tail []byte, err error) {
	if len(data) < 2 {
		err = Error("not enough bytes to parse an object")
		return
	}
	o.ID = ID(data[0])
	o.Data = nil
	n := int(data[1])
	data = data[2:]
	if len(data) < n {
		err = Error("invalid number of object bytes")
		return
	}
	tail = data[n:]
	if n == 0 {
		return
	}
	o.Data = make([]byte, n)
	copy(o.Data, data)
	return
}
