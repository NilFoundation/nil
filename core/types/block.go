package types

import (
	common "github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type Block struct {
	Id                  uint64
	PrevBlock           common.Hash
	SmartContractsRoot  common.Hash
	MessagesRoot        common.Hash
	ReceiptsRoot        common.Hash
	ChildBlocksRootHash common.Hash
	MasterChainHash     common.Hash
	LogsBloom           Bloom
}

// interfaces
var (
	_ common.Hashable     = new(Block)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (b *Block) Hash() common.Hash {
	h, err := common.PoseidonSSZ(b)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't get block hash")
	}
	return h
}
