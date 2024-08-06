package common

import (
	"hash"
	"sync"

	"github.com/NilFoundation/nil/nil/common/check"
	"golang.org/x/crypto/sha3"
)

var cryptoPool = sync.Pool{
	New: func() interface{} {
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
