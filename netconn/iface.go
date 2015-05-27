package netconn

import (
	"fmt"
	"io"
	"sort"
)

type Interface struct {
	Name string
	Desc string
	Elem interface{}
}

type InterfaceGroup struct {
	Name       string
	Type       string
	Interfaces func() []Interface
	SortPrefix string
	Hidden     bool
}

func Interfaces() []*InterfaceGroup {
	m := make(map[string]*InterfaceGroup, len(protos))
	for _, p := range protos {
		ig := p.InterfaceGroup
		m[ig.SortPrefix+ig.Name] = ig
	}

	n := len(m)
	names := make([]string, 0, n)
	for groupName, _ := range m {
		names = append(names, groupName)
	}
	sort.Strings(names)

	list := make([]*InterfaceGroup, 0, n)
	for _, groupName := range names {
		list = append(list, m[groupName])
	}
	return list
}

func FprintInterfaces(w io.Writer, all bool) {
	prevOutput := false
	for _, g := range Interfaces() {
		if g.Hidden && !all {
			continue
		}
		ifaces := g.Interfaces()
		if len(ifaces) == 0 {
			continue
		}
		if prevOutput {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, g.Name+":")
		for _, iface := range g.Interfaces() {
			fmt.Fprintf(w, "\t%s\t(%s)\n", iface.Name, iface.Desc)
		}
		prevOutput = true
	}
}
