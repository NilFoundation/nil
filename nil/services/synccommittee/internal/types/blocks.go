package types

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BlockRef struct {
	ShardId types.ShardId     `json:"shardId"`
	Hash    common.Hash       `json:"hash"`
	Number  types.BlockNumber `json:"number"`
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

type BlockId struct {
	ShardId types.ShardId
	Hash    common.Hash
}

func NewBlockId(shardId types.ShardId, hash common.Hash) BlockId {
	return BlockId{shardId, hash}
}

func IdFromBlock(block *jsonrpc.RPCBlock) BlockId {
	return BlockId{block.ShardId, block.Hash}
}

func (bk BlockId) Bytes() []byte {
	key := make([]byte, 4+common.HashSize)
	binary.LittleEndian.PutUint32(key[:4], uint32(bk.ShardId))
	copy(key[4:], bk.Hash.Bytes())
	return key
}

func (bk BlockId) String() string {
	return hex.EncodeToString(bk.Bytes())
}
