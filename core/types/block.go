package types

import (
	"log"

	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
)

type Block struct {
	Id                 uint64      `json:"Id"`
	PrevBlock          common.Hash `json:"PrevBlock"`
	SmartContractsRoot common.Hash `json:"smartContractsRoot"`
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

func (b *Block) Hash() common.Hash {
	h, err := ssz.SSZHash(b)
	if err != nil {
		log.Fatal(err)
	}
	return h
}
