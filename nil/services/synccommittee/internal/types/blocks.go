package types

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

// MainBlockRef represents a reference to a specific main shard block
type MainBlockRef struct {
	Hash   common.Hash       `json:"hash"`
	Number types.BlockNumber `json:"number"`
}

func NewBlockRef(block *jsonrpc.RPCBlock) (*MainBlockRef, error) {
	if block == nil {
		return nil, errors.New("block cannot be nil")
	}

	if block.ShardId != types.MainShardId {
		return nil, fmt.Errorf("block is not from main shard: %d", block.ShardId)
	}

	return &MainBlockRef{
		Hash:   block.Hash,
		Number: block.Number,
	}, nil
}

type BlocksRange struct {
	Start types.BlockNumber
	End   types.BlockNumber
}

func GetBlocksFetchingRange(latestFetched *MainBlockRef, actualLatest MainBlockRef) (*BlocksRange, error) {
	switch {
	case latestFetched == nil:
		return &BlocksRange{actualLatest.Number, actualLatest.Number}, nil

	case latestFetched.Number < actualLatest.Number:
		return &BlocksRange{latestFetched.Number + 1, actualLatest.Number}, nil

	case latestFetched.Number == actualLatest.Number && latestFetched.Hash != actualLatest.Hash:
		return nil, fmt.Errorf(
			"%w: latest blocks have same number %d, but hashes are different: %s != %s",
			ErrBlockMismatch, actualLatest.Number, latestFetched.Hash, actualLatest.Hash,
		)

	case latestFetched.Number > actualLatest.Number:
		return nil, fmt.Errorf(
			"%w: latest fetched block is higher than actual latest block: %d > %d",
			ErrBlockMismatch, latestFetched.Number, actualLatest.Number,
		)

	default:
		return nil, nil
	}
}

func (br *MainBlockRef) Equals(child *jsonrpc.RPCBlock) bool {
	if br == nil || child == nil {
		return br == nil && child == nil
	}

	return br.Hash == child.Hash && br.Number == child.Number
}

func (br *MainBlockRef) ValidateChild(child *jsonrpc.RPCBlock) error {
	switch {
	case br == nil:
		return nil

	case child == nil:
		return errors.New("child block cannot be nil")

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

func ChildBlockIds(mainShardBlock *jsonrpc.RPCBlock) ([]BlockId, error) {
	if mainShardBlock == nil {
		return nil, errors.New("mainShardBlock cannot be nil")
	}

	if mainShardBlock.ShardId != types.MainShardId {
		return nil, fmt.Errorf("mainShardBlock is not from the main shard: %d", mainShardBlock.ShardId)
	}

	blockIds := make([]BlockId, 0, len(mainShardBlock.ChildBlocks))

	for i, childHash := range mainShardBlock.ChildBlocks {
		shardId := types.ShardId(i + 1)
		blockId := NewBlockId(shardId, childHash)
		blockIds = append(blockIds, blockId)
	}

	return blockIds, nil
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

type PrunedTransaction struct {
	Flags types.MessageFlags
	Seqno hexutil.Uint64
	From  types.Address
	To    types.Address
	Value types.Value
	Data  hexutil.Bytes
}

func NewTransaction(message *jsonrpc.RPCInMessage) *PrunedTransaction {
	return &PrunedTransaction{
		Flags: message.Flags,
		Seqno: message.Seqno,
		From:  message.From,
		To:    message.To,
		Value: message.Value,
		Data:  message.Data,
	}
}

type ProposalData struct {
	MainShardBlockHash common.Hash
	Transactions       []*PrunedTransaction
	OldProvedStateRoot common.Hash
	NewProvedStateRoot common.Hash
	MainBlockFetchedAt time.Time
}
