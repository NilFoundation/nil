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

func (br *MainBlockRef) String() string {
	return fmt.Sprintf("BlockRef{hash=%s, number=%d}", br.Hash, br.Number)
}

type BlocksRange struct {
	Start types.BlockNumber
	End   types.BlockNumber
}

// GetBlocksFetchingRange determines the range of blocks to fetch between the latest handled block
// and the actual latest block.
// latestHandled can be equal to:
// a) The latest block fetched from the cluster, or
// b) The latest proved state root, if `latestFetched` is nil.
func GetBlocksFetchingRange(
	latestHandled MainBlockRef,
	actualLatest MainBlockRef,
	maxNumBlocks uint32,
) (*BlocksRange, error) {
	if maxNumBlocks == 0 {
		return nil, nil
	}

	var blocksRange BlocksRange
	switch {
	case latestHandled.Number < actualLatest.Number:
		blocksRange = BlocksRange{latestHandled.Number + 1, actualLatest.Number}

	case latestHandled.Number == actualLatest.Number && latestHandled.Hash != actualLatest.Hash:
		return nil, fmt.Errorf(
			"%w: latest blocks have same number %d, but hashes are different: %s != %s",
			ErrBlockMismatch, actualLatest.Number, latestHandled.Hash, actualLatest.Hash,
		)

	case latestHandled.Number > actualLatest.Number:
		return nil, fmt.Errorf(
			"%w: latest fetched block is higher than actual latest block: %d > %d",
			ErrBlockMismatch, latestHandled.Number, actualLatest.Number,
		)

	default:
		return nil, nil
	}

	rangeSize := uint32(blocksRange.End - blocksRange.Start + 1)
	if rangeSize <= maxNumBlocks {
		return &blocksRange, nil
	}

	return &BlocksRange{blocksRange.Start, blocksRange.Start + types.BlockNumber(maxNumBlocks-1)}, nil
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

func GetMainParentRef(mainBlock *jsonrpc.RPCBlock) (*MainBlockRef, error) {
	if mainBlock == nil {
		return nil, errors.New("mainBlock cannot be nil")
	}
	if mainBlock.ShardId != types.MainShardId {
		return nil, fmt.Errorf("mainBlock is not from main shard: %d", mainBlock.ShardId)
	}
	if mainBlock.Number == 0 || mainBlock.ParentHash.Empty() {
		return nil, nil
	}
	return &MainBlockRef{
		Number: mainBlock.Number - 1,
		Hash:   mainBlock.ParentHash,
	}, nil
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

type PrunedBlock struct {
	ShardId       types.ShardId
	BlockNumber   types.BlockNumber
	Timestamp     uint64
	PrevBlockHash common.Hash
	Transactions  []*PrunedTransaction
}

func NewPrunedBlock(block *jsonrpc.RPCBlock) *PrunedBlock {
	return &PrunedBlock{
		ShardId:       block.ShardId,
		BlockNumber:   block.Number,
		Timestamp:     block.DbTimestamp,
		PrevBlockHash: block.ParentHash,
		Transactions:  BlockTransactions(block),
	}
}

type PrunedTransaction struct {
	Flags    types.TransactionFlags
	Seqno    hexutil.Uint64
	From     types.Address
	To       types.Address
	BounceTo types.Address
	RefundTo types.Address
	Value    types.Value
	Data     hexutil.Bytes
}

func BlockTransactions(block *jsonrpc.RPCBlock) []*PrunedTransaction {
	transactions := make([]*PrunedTransaction, len(block.Transactions))
	for idx, transaction := range block.Transactions {
		transactions[idx] = NewTransaction(transaction)
	}
	return transactions
}

func NewTransaction(transaction *jsonrpc.RPCInTransaction) *PrunedTransaction {
	return &PrunedTransaction{
		Flags:    transaction.Flags,
		Seqno:    transaction.Seqno,
		From:     transaction.From,
		To:       transaction.To,
		BounceTo: transaction.BounceTo,
		RefundTo: transaction.RefundTo,
		Value:    transaction.Value,
		Data:     transaction.Data,
	}
}

type ProposalData struct {
	BatchId            BatchId
	MainShardBlockHash common.Hash
	Transactions       []*PrunedTransaction
	OldProvedStateRoot common.Hash
	NewProvedStateRoot common.Hash
	MainBlockFetchedAt time.Time
}
