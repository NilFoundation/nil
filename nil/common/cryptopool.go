package common

import (
	"hash"
	"sync"

	"github.com/NilFoundation/nil/nil/common/check"
	"golang.org/x/crypto/sha3"
)

var cryptoPool = sync.Pool{
	New: func() any {
		return sha3.NewLegacyKeccak256()
	},
}

func GetLegacyKeccak256() hash.Hash {
	h, ok := cryptoPool.Get().(hash.Hash)
	check.PanicIfNot(ok)
	h.Reset()
	return h
}
func ReturnLegacyKeccak256(h hash.Hash) { cryptoPool.Put(h) }

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) Hash {
	sha := GetLegacyKeccak256()
	for _, b := range data {
		sha.Write(b)
	}
	h := sha.Sum(nil)
	ReturnLegacyKeccak256(sha)
	return BytesToHash(h)
}
