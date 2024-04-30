package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
)

type Block struct {
	Id                 uint64
	PrevBlock          common.Hash
	SmartContractsRoot common.Hash
}

// interfaces
var _ ssz.SSZEncodable = new(Block)
var _ common.Hashable = new(SmartContract)

func (b *Block) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		ssz.Uint64SSZ(b.Id),
		b.PrevBlock[:],
		b.SmartContractsRoot[:],
	)
}

func (b *Block) DecodeSSZ(buf []byte, version int) error {
	return ssz.UnmarshalSSZ(
		buf,
		0,
		&b.Id,
		b.PrevBlock[:],
		b.SmartContractsRoot[:],
	)
}

func (b *Block) EncodingSizeSSZ() int {
	return common.Bytes64Size + common.HashSize + common.HashSize
}

func (b *Block) Hash() common.Hash {
	h, err := ssz.SSZHash(b)
	if err != nil {
		common.DefaultLogger.Fatal().Msg(err.Error())
	}
	return h
}
