package types

import (
	"slices"

	"github.com/NilFoundation/nil/common"
)

type Code []byte

var _ common.Hashable = new(Code)

func (c Code) String() string {
	return string(c)
}

func (s Code) Clone() common.Clonable {
	cloned := slices.Clone(s)
	return &cloned
}

func (c Code) Hash() common.Hash {
	if len(c) == 0 {
		return common.EmptyHash
	}
	return common.PoseidonHash(c[:])
}

func (s Code) Static() bool {
	return false
}
