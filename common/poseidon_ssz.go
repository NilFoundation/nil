package common

import (
	"sync"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/rs/zerolog/log"
)

// HasherPool may be used for pooling Hasher`s for similarly typed SSZs.
type HasherPool struct {
	pool sync.Pool
}

// Get acquires a Hasher from the pool.
func (hh *HasherPool) Get() *ssz.Hasher {
	h := hh.pool.Get()
	if h == nil {
		hash, err := poseidon.New(16)
		FatalIf(err, log.Logger, "Can't create poseidon hasher")

		return ssz.NewHasherWithHash(hash)
	}

	res, ok := h.(*ssz.Hasher)
	Require(ok)
	return res
}

// Put releases the Hasher to the pool.
func (hh *HasherPool) Put(h *ssz.Hasher) {
	h.Reset()
	hh.pool.Put(h)
}

// DefaultHasherPool is a default hasher pool
var DefaultHasherPool HasherPool

func PoseidonSSZ(v ssz.HashRoot) (Hash, error) {
	hh := DefaultHasherPool.Get()
	if err := v.HashTreeRootWith(hh); err != nil {
		DefaultHasherPool.Put(hh)
		return [32]byte{}, err
	}
	root, err := hh.HashRoot()
	DefaultHasherPool.Put(hh)
	return root, err
}
