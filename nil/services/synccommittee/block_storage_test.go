package synccommittee

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

func TestNewBlockStorage(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	if bs == nil {
		t.Fatal("NewBlockStorage returned nil")
	}
	if bs.blocksStorage == nil {
		t.Error("blocksStorage map not initialized")
	}
	if bs.lastFetchedBlockNumPerShard == nil {
		t.Error("lastFetchedBlockNumPerShard map not initialized")
	}
	if bs.lastProvedBlockNumPerShard == nil {
		t.Error("lastProvedBlockNumPerShard map not initialized")
	}
}

func TestGetSetBlock(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	blockNumber := types.BlockNumber(10)
	block := &jsonrpc.RPCBlock{Number: blockNumber, ShardId: shardId}

	// Test SetBlock
	bs.SetBlock(block)

	// Test GetBlock
	retrievedBlock := bs.GetBlock(shardId, blockNumber)
	if retrievedBlock != block {
		t.Errorf("GetBlock returned incorrect block. Expected %v, got %v", block, retrievedBlock)
	}

	// Test GetBlock for non-existent block
	nonExistentBlock := bs.GetBlock(shardId, blockNumber+1)
	if nonExistentBlock != nil {
		t.Errorf("GetBlock returned non-nil for non-existent block. Got %v", nonExistentBlock)
	}
}

func TestGetSetLastFetchedBlockNum(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	block1 := &jsonrpc.RPCBlock{Number: 10, ShardId: shardId}
	block2 := &jsonrpc.RPCBlock{Number: 20, ShardId: shardId}

	bs.SetBlock(block1)
	bs.SetBlock(block2)

	// Test GetLastFetchedBlock
	lastFetchedBlockNum := bs.GetLastFetchedBlockNum(shardId)
	if lastFetchedBlockNum != block2.Number {
		t.Errorf("GetLastFetchedBlock returned incorrect block. Expected %d, got %d", block2.Number, lastFetchedBlockNum)
	}

	// Test SetLastFetchedBlockNum
	bs.SetLastFetchedBlockNum(shardId, block1.Number)

	lastFetchedBlockNum = bs.GetLastFetchedBlockNum(shardId)
	if lastFetchedBlockNum != block1.Number {
		t.Errorf("GetLastFetchedBlock returned incorrect block. Expected %d, got %d", block1.Number, lastFetchedBlockNum)
	}
}

func TestGetSetLastProvedBlockNum(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	blockNum := types.BlockNumber(100)

	bs.SetLastProvedBlockNum(shardId, blockNum)
	lastProved := bs.GetLastProvedBlockNum(shardId)

	if lastProved != blockNum {
		t.Errorf("GetLastProvedBlockNum returned incorrect number. Expected %d, got %d", blockNum, lastProved)
	}
}

func TestGetBlocksRange(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	for i := types.BlockNumber(1); i <= 10; i++ {
		bs.SetBlock(&jsonrpc.RPCBlock{Number: i, ShardId: shardId})
	}

	blocks := bs.GetBlocksRange(shardId, 3, 8)
	if len(blocks) != 5 {
		t.Errorf("GetBlocksRange returned incorrect number of blocks. Expected 5, got %d", len(blocks))
	}

	for i, block := range blocks {
		expectedNumber := types.BlockNumber(i + 3)
		if block.Number != expectedNumber {
			t.Errorf("Block at index %d has incorrect number. Expected %d, got %d", i, expectedNumber, block.Number)
		}
	}

	// Test empty range
	emptyBlocks := bs.GetBlocksRange(shardId, 11, 15)
	if len(emptyBlocks) != 0 {
		t.Errorf("GetBlocksRange returned non-empty slice for empty range. Got %d blocks", len(emptyBlocks))
	}
}

func TestGetTransactionsByBlocksRange(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	for i := types.BlockNumber(1); i <= 10; i++ {
		message := jsonrpc.RPCInMessage{Seqno: hexutil.Uint64(i)}
		messages := make([]any, 1)
		messages[0] = message
		block := jsonrpc.RPCBlock{Number: i, ShardId: shardId, Messages: messages}
		bs.SetBlock(&block)
	}

	transactions := bs.GetTransactionsByBlocksRange(shardId, 3, 8)
	if len(transactions) != 5 {
		t.Errorf("GetTransactionsByBlocksRange returned incorrect number of transactions. Expected 5, got %d", len(transactions))
	}

	for i, transaction := range transactions {
		expectedSeqno := hexutil.Uint64(i + 3)
		if transaction.seqno != expectedSeqno {
			t.Errorf("transaction at index %d has incorrect seqno. Expected %d, got %d", i, expectedSeqno, transaction.seqno)
		}
	}

	// Test empty range
	emptyTransactions := bs.GetTransactionsByBlocksRange(shardId, 11, 15)
	if len(emptyTransactions) != 0 {
		t.Errorf("GetTransactionsByBlocksRange returned non-empty slice for empty range. Got %d transactions", len(emptyTransactions))
	}
}

func TestCleanupStorage(t *testing.T) {
	t.Parallel()
	bs := NewBlockStorage()
	shardId := types.MainShardId
	for i := types.BlockNumber(1); i <= 10; i++ {
		bs.SetBlock(&jsonrpc.RPCBlock{Number: i, ShardId: shardId})
	}

	bs.SetLastProvedBlockNum(shardId, 5)
	bs.CleanupStorage()

	for i := types.BlockNumber(1); i <= 10; i++ {
		block := bs.GetBlock(shardId, i)
		if i < 5 && block != nil {
			t.Errorf("Block %d should have been cleaned up, but it still exists", i)
		} else if i >= 5 && block == nil {
			t.Errorf("Block %d should not have been cleaned up, but it doesn't exist", i)
		}
	}
}
