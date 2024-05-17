package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

type Message struct {
	Index     uint64         `json:"index,omitempty"`
	ShardId   ShardId        `json:"shard,omitempty"`
	From      common.Address `json:"from,omitempty"`
	To        common.Address `json:"to,omitempty"`
	Value     uint256.Int    `json:"value,omitempty"`
	Data      Code           `json:"data,omitempty"`
	Seqno     uint64         `json:"seqno,omitempty"`
	Signature common.Hash    `json:"signature,omitempty"`
}

type MessageId struct {
	BlockId      uint64
	MessageIndex uint64
}

// interfaces
var _ common.Hashable = new(Message)
var _ ssz.SSZEncodable = new(Message)
var _ ssz.SSZDecodable = new(Message)

var _ common.Hashable = new(MessageId)
var _ ssz.SSZEncodable = new(MessageId)
var _ ssz.SSZDecodable = new(MessageId)

func (s *Message) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		&s.ShardId,
		s.From[:],
		s.To[:],
		ssz.Uint256SSZ(s.Value),
		&s.Data,
		s.Seqno,
		s.Signature[:],
	)
}

func (s *Message) EncodingSizeSSZ() int {
	return common.Uint64Size + s.ShardId.EncodingSizeSSZ() + common.AddrSize + common.AddrSize + common.Bits256Size + s.Data.EncodingSizeSSZ() + common.HashSize
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
		&s.ShardId,
		s.From[:],
		s.To[:],
		value,
		&s.Data,
		&s.Seqno,
		s.Signature[:],
	)

	if err != nil {
		return err
	}

	if err := s.Value.UnmarshalSSZ(value); err != nil {
		return err
	}

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

func (s *MessageId) Clone() common.Clonable {
	clonned := *s
	return &clonned
}

func (s *MessageId) EncodingSizeSSZ() int {
	return common.Uint64Size * 2
}

func (s *MessageId) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		&s.BlockId,
		&s.MessageIndex,
	)
}

func (s *MessageId) DecodeSSZ(buf []byte, version int) error {
	return ssz.UnmarshalSSZ(
		buf,
		0,
		&s.BlockId,
		&s.MessageIndex,
	)
}

func (s *MessageId) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func DecodeMessage(raw []byte) (*Message, error) {
	var msg Message
	if err := msg.DecodeSSZ(raw, 0); err != nil {
		return nil, err
	}
	return &msg, nil
}
