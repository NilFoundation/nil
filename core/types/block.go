package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
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
var _ common.Hashable = new(Block)

func (b *Block) Hash() common.Hash {
	h, err := ssz.FastSSZHash(b)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}
