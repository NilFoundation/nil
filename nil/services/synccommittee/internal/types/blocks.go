package types

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

// BlockRef represents a reference to a specific shard block
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

type BlocksRange struct {
	Start types.BlockNumber
	End   types.BlockNumber
}

func GetBlocksFetchingRange(latestFetched *BlockRef, actualLatest BlockRef) (*BlocksRange, error) {
	switch {
	case latestFetched == nil:
		return &BlocksRange{actualLatest.Number, actualLatest.Number}, nil

	case latestFetched.ShardId != actualLatest.ShardId:
		return nil, fmt.Errorf(
			"%w: shard id mismatch: %d != %d", ErrBlockMismatch, latestFetched.ShardId, actualLatest.ShardId,
		)

	case latestFetched.Number < actualLatest.Number:
		return &BlocksRange{latestFetched.Number + 1, actualLatest.Number}, nil

	case latestFetched.Number == actualLatest.Number && latestFetched.Hash != actualLatest.Hash:
		return nil, fmt.Errorf(
			"%w: latest blocks have same number %d, but hashes are different: %s != %s",
			ErrBlockMismatch, actualLatest.Number, latestFetched.Hash, actualLatest.Hash,
		)

	case latestFetched.Number > actualLatest.Number:
		return nil, fmt.Errorf(
			"%w: latest fetched block for shard %d is higher than actual latest block: %d > %d",
			ErrBlockMismatch, latestFetched.ShardId, latestFetched.Number, actualLatest.Number,
		)

	default:
		return nil, nil
	}
}

func (br *BlockRef) Equals(child *jsonrpc.RPCBlock) bool {
	return br != nil &&
		child != nil &&
		br.ShardId == child.ShardId &&
		br.Hash == child.Hash &&
		br.Number == child.Number
}

func (br *BlockRef) ValidateChild(child *jsonrpc.RPCBlock) error {
	switch {
	case br == nil:
		return nil

	case child == nil:
		return errors.New("child block cannot be nil")

	case child.ShardId != br.ShardId:
		return fmt.Errorf("%w: shard id mismatch: %d != %d", ErrBlockMismatch, br.ShardId, child.ShardId)

	case child.Number != br.Number+1:
		return fmt.Errorf(
			"%w: [hash=%s] block number mismatch: expected=%d, got=%d",
			ErrBlockMismatch, child.Hash, br.Number+1, child.Number,
		)

	case child.ParentHash != br.Hash:
		return fmt.Errorf(
			"%w: [hash=%s] parent hash mismatch: expected=%s, got=%s",
			ErrBlockMismatch, child.Hash, br.Hash, child.ParentHash,
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
