package msgpool

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
)

const defaultPoolSize = 10000

type Config struct {
	ShardId types.ShardId
	Size    uint64
}

func NewConfig(shardId types.ShardId) Config {
	return Config{
		ShardId: shardId,
		Size:    defaultPoolSize,
	}
}

type DiscardReason uint8

const (
	NotSet              DiscardReason = 0 // analog of "nil-value", means it will be set in future
	Success             DiscardReason = 1
	AlreadyKnown        DiscardReason = 2
	Committed           DiscardReason = 3
	ReplacedByHigherTip DiscardReason = 4
	NegativeValue       DiscardReason = 10 // ensure no one is able to specify a transaction with a negative value.
	PoolOverflow        DiscardReason = 12
	SeqnoTooLow         DiscardReason = 18
	NotReplaced         DiscardReason = 20 // There was an existing transaction with the same sender and seqno, not enough price bump to replace
	DuplicateHash       DiscardReason = 21 // There was an existing message with the same hash
)

func (r DiscardReason) String() string {
	switch r {
	case NotSet:
		return "not set"
	case Success:
		return "success"
	case AlreadyKnown:
		return "already known"
	case Committed:
		return "committed"
	case ReplacedByHigherTip:
		return "replaced by higher tip"
	case NotReplaced:
		return "not replaced"
	case NegativeValue:
		return "negative value"
	case PoolOverflow:
		return "pool overflow"
	case SeqnoTooLow:
		return "seqno too low"
	case DuplicateHash:
		return "duplicate hash"
	default:
		panic(fmt.Sprintf("discard reason: %d", r))
	}
}
