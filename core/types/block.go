package types

import (
	"strconv"

	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type BlockNumber uint64

func (bn BlockNumber) Uint64() uint64 {
	return uint64(bn)
}

func (bn BlockNumber) String() string { return strconv.FormatUint(bn.Uint64(), 10) }
func (bn BlockNumber) Bytes() []byte  { return []byte(bn.String()) }

type Block struct {
	Id                  BlockNumber
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
