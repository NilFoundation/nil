package types

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BlockRef struct {
	ShardId types.ShardId
	Hash    common.Hash
	Number  types.BlockNumber
}

func NewBlockRef(block *jsonrpc.RPCBlock) BlockRef {
	return BlockRef{
		ShardId: block.ShardId,
		Hash:    block.Hash,
		Number:  block.Number,
	}
}

func (parent *BlockRef) ValidateChild(child BlockRef) error {
	switch {
	case parent == nil:
		return nil

	case parent.ShardId != child.ShardId:
		return fmt.Errorf("%w: shard id mismatch: %d != %d", ErrBlockMismatch, parent.ShardId, child.ShardId)

	case parent.Number == child.Number && parent.Hash != child.Hash:
		return &BlockHashMismatchError{
			Expected: parent.Hash, Got: child.Hash, LatestFetched: parent.Number,
		}

	case parent.Number > child.Number:
		return fmt.Errorf(
			"%w: latest fetched block for shard %d is higher than actual latest block: %d > %d",
			ErrBlockMismatch, parent.ShardId, parent.Number, child.Number,
		)

	default:
		return nil
	}
}
