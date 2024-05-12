package common

import (
	"golang.org/x/crypto/sha3"
	"hash"
	"sync"
)

var cryptoPool = sync.Pool{
	New: func() interface{} {
		return sha3.NewLegacyKeccak256()
	},
}

func GetLegacyKeccak256() hash.Hash {
	h := cryptoPool.Get().(hash.Hash)
	h.Reset()
	return h
}
func ReturnLegacyKeccak256(h hash.Hash) { cryptoPool.Put(h) }
