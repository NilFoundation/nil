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
	ChildBlocksRootHash common.Hash
	MasterChainHash     common.Hash
}

// interfaces
var _ common.Hashable = new(Block)
var _ ssz.SSZEncodable = new(Block)
var _ ssz.SSZDecodable = new(Block)

func (b *Block) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		ssz.Uint64SSZ(b.Id),
		b.PrevBlock[:],
		b.SmartContractsRoot[:],
		b.MessagesRoot[:],
		b.ChildBlocksRootHash[:],
		b.MasterChainHash[:],
	)
}

func (b *Block) EncodingSizeSSZ() int {
	return common.Uint64Size + 5*common.HashSize
}

func (b *Block) Clone() common.Clonable {
	clonned := *b
	return &clonned
}

func (b *Block) DecodeSSZ(buf []byte, version int) error {
	return ssz.UnmarshalSSZ(
		buf,
		0,
		&b.Id,
		b.PrevBlock[:],
		b.SmartContractsRoot[:],
		b.MessagesRoot[:],
		b.ChildBlocksRootHash[:],
		b.MasterChainHash[:],
	)
}

func (b *Block) Hash() common.Hash {
	h, err := ssz.SSZHash(b)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func (s *Block) Static() bool {
	return true
}
