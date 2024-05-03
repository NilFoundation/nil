package types

import (
	"github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type Code []byte

var _ ssz.SizedObjectSSZ = new(Code)

func (c Code) String() string {
	return string(c)
}

func (c Code) EncodingSizeSSZ() int {
	return len(c)
}

func (s *Code) Clone() common.Clonable {
	clonned := *s
	return &clonned
}

func (s *Code) DecodeSSZ(buf []byte, version int) error {
	*s = buf
	return nil
}

func (s *Code) EncodeSSZ(dst *[]byte) error {
	*dst = append(*dst, *s...)
	return nil
}

func (c *Code) Hash() common.Hash {
	return common.CastToHash(poseidon.Sum((*c)[:]))
}

func (s *Code) Static() bool {
	return false
}
