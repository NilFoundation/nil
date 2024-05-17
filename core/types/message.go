package types

import (
	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type Message struct {
	Index     uint64         `json:"index,omitempty"`
	ShardId   ShardId        `json:"shard,omitempty"`
	From      common.Address `json:"from,omitempty"`
	To        common.Address `json:"to,omitempty"`
	Value     Uint256        `json:"value,omitempty"`
	Data      Code           `json:"data,omitempty" ssz-max:"10000"`
	Seqno     uint64         `json:"seqno,omitempty"`
	Signature common.Hash    `json:"signature,omitempty"`
}

// interfaces
var (
	_ common.Hashable     = new(Message)
	_ fastssz.Marshaler   = new(Message)
	_ fastssz.Unmarshaler = new(Message)
)

func (m *Message) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't get message hash")
	}
	return h
}
