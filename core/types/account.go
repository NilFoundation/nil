package types

import (
	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
)

type SmartContract struct {
	Address     common.Address
	Initialised bool
	Balance     Uint256 `ssz-size:"32"`
	StorageRoot common.Hash
	CodeHash    common.Hash
	Seqno       uint64
	PublicKey   []byte `ssz-max:"33"`
}

// interfaces
var (
	_ common.Hashable     = new(SmartContract)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	h, err := common.PoseidonSSZ(s)
	common.FatalIf(err, nil, "Can't get smart contract hash")

	return h
}
