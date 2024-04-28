package types

import (
	"github.com/NilFoundation/nil/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type Code []byte

func (c Code) String() string {
	return string(c)
}

func (c *Code) Hash() common.Hash {
	return common.CastToHash(poseidon.Sum((*c)[:]))
}
