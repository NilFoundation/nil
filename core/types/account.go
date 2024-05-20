package types

import (
	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type SmartContract struct {
	Address     common.Address
	Initialised bool
	Balance     Uint256
	StorageRoot common.Hash
	CodeHash    common.Hash
	Seqno       uint64
}

// interfaces
var (
	_ common.Hashable     = new(SmartContract)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	h, err := common.PoseidonSSZ(s)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't get smart contract hash")
	}
	return h
}
