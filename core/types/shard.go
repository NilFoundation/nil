package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/rs/zerolog/log"
)

type Shard struct {
	Id           uint64
	GenesisBlock common.Hash
}

// interfaces
var _ ssz.SizedObjectSSZ = new(Shard)
var _ common.Hashable = new(Shard)

func (s *Shard) EncodeSSZ(dst *[]byte) error {
	return ssz.MarshalSSZ(
		dst,
		ssz.Uint64SSZ(s.Id),
		s.GenesisBlock[:],
	)
}

func (s *Shard) EncodingSizeSSZ() int {
	return common.Uint64Size + common.HashSize
}

func (s *Shard) Clone() common.Clonable {
	clonned := *s
	return &clonned
}

func (s *Shard) DecodeSSZ(buf []byte, version int) error {
	err := ssz.UnmarshalSSZ(
		buf,
		0,
		&s.Id,
		s.GenesisBlock[:],
	)

	if err != nil {
		return err
	}
	return nil
}

func (s *Shard) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func (s *Shard) Static() bool {
	return true
}
