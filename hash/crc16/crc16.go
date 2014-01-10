// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crc16

import (
	"modbus/hash"
	"sync"
)

// The size of a CRC-16 checksum in bytes.
const Size = 2

// Predefined polynomials.
const (
	// IBM-CRC-16, reversed polynomial of 0x8005
	IBMCRC = 0xA001
)

// Table is a 256-word table representing the polynomial for efficient processing.
type Table [256]uint16

var ibmcrcTable *Table
var ibmcrcOnce sync.Once

func ibmcrcInit() {
	ibmcrcTable = makeTable(IBMCRC)
}

// MakeTable returns the Table constructed from the specified polynomial.
func MakeTable(poly uint16) *Table {
	switch poly {
	case IBMCRC:
		ibmcrcOnce.Do(ibmcrcInit)
		return ibmcrcTable
	}
	return makeTable(poly)
}

// makeTable returns the Table constructed from the specified polynomial.
func makeTable(poly uint16) *Table {
	t := new(Table)
	for i := 0; i < 256; i++ {
		crc := uint16(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
	return t
}

// digest represents the partial evaluation of a checksum.
type digest struct {
	crc uint16
	tab *Table
}

// New creates a new hash.Hash16 computing the CRC-16 checksum
// using the polynomial represented by the Table.
func New(tab *Table) hash.Hash16 { return &digest{0, tab} }

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return 1 }

func (d *digest) Reset() { d.crc = 0 }

func update(crc uint16, tab *Table, p []byte) uint16 {
	crc = ^crc
	for _, v := range p {
		crc = tab[byte(crc)^v] ^ (crc >> 8)
	}
	return ^crc
}

// Update returns the result of adding the bytes in p to the crc.
func Update(crc uint16, tab *Table, p []byte) uint16 {
	return update(crc, tab, p)
}

func (d *digest) Write(p []byte) (n int, err error) {
	d.crc = Update(d.crc, d.tab, p)
	return len(p), nil
}

func (d *digest) Sum16() uint16 {
	if d.tab == ibmcrcTable {
		return ^d.crc
	}
	return d.crc
}

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum16()
	return append(in, byte(s>>8), byte(s))
}

// Checksum returns the CRC-16 checksum of data
// using the polynomial represented by the Table.
func Checksum(data []byte, tab *Table) (crc uint16) {
	crc = Update(0, tab, data)
	if tab == ibmcrcTable {
		crc = ^crc
	}
	return crc
}
