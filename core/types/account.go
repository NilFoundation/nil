package types

import (
	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/rs/zerolog/log"
)

// PublicKeySize is the expected length of the PublicKey (in bytes)
const PublicKeySize = 33

var EmptyPublicKey [PublicKeySize]byte

type SmartContract struct {
	Address     Address
	Initialised bool
	Balance     Uint256 `ssz-size:"32"`
	StorageRoot common.Hash
	CodeHash    common.Hash
	Seqno       Seqno
	PublicKey   [PublicKeySize]byte
}

// interfaces
var (
	_ common.Hashable     = new(SmartContract)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	h, err := common.PoseidonSSZ(s)
	common.FatalIf(err, log.Logger, "Can't get smart contract hash")

	return h
}
