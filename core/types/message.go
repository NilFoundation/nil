package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

type Message struct {
	ShardInfo Shard
	From      common.Address
	To        common.Address
	Value     uint256.Int
	Data      Code
	Signature common.Hash
}

// interfaces
var _ common.Hashable = new(Message)
var _ ssz.SSZEncodable = new(Message)
var _ ssz.SSZDecodable = new(Message)

func (s *Message) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		&s.ShardInfo,
		s.From[:],
		s.To[:],
		ssz.Uint256SSZ(s.Value),
		&s.Data,
		s.Signature[:],
	)
}

func (s *Message) EncodingSizeSSZ() int {
	return s.ShardInfo.EncodingSizeSSZ() + common.AddrSize + common.AddrSize + common.Bits256Size + s.Data.EncodingSizeSSZ() + common.HashSize
}

func (s *Message) Clone() common.Clonable {
	clonned := *s
	return &clonned
}

func (s *Message) DecodeSSZ(buf []byte, version int) error {
	value := make([]byte, common.Bits256Size)
	err := ssz.UnmarshalSSZ(
		buf,
		0,
		&s.ShardInfo,
		s.From[:],
		s.To[:],
		value,
		&s.Data,
		s.Signature[:],
	)

	if err != nil {
		return err
	}

	s.Value.SetBytes(value)

	return nil
}

func (s *Message) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func (s *Message) Static() bool {
	return false
}
