package hash

import (
	"hash"
)

// Hash16 is the common interface implemented by all 16-bit hash functions.
type Hash16 interface {
	hash.Hash
	Sum16() uint16
}
