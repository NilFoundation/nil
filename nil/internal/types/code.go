package types

import (
	"encoding/hex"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type Code []byte

var _ common.Hashable = new(Code)

func (c Code) String() string {
	return string(c)
}

func (c Code) Clone() Code {
	return slices.Clone(c)
}

func (c Code) Hash() common.Hash {
	if len(c) == 0 {
		return common.EmptyHash
	}
	return common.PoseidonHash(c[:])
}

func (c Code) Hex() string {
	enc := make([]byte, hex.EncodedLen(len(c))+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], c[:])
	return string(enc)
}

func (c Code) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.Hex() + `"`), nil
}

func (c *Code) UnmarshalJSON(input []byte) error {
	b := (*hexutil.Bytes)(c)
	return b.UnmarshalJSON(input)
}
