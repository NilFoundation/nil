package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
)

type Block struct {
	Id                 uint64      `hashable:"" storable:""`
	PrevBlock          common.Hash `hashable:"" storable:""`
	SmartContractsRoot common.Hash `hashable:"" storable:""`

	SmartContracts *common.TreeWrapper
}

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
