//go:build test

package testaide

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/holiman/uint256"
)

const (
	ShardsCount = 4
	BatchSize   = ShardsCount + 1
)

func RandomHash() common.Hash {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return common.BytesToHash(randomBytes)
}

func RandomBlockNum() types.BlockNumber {
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return types.BlockNumber(binary.LittleEndian.Uint64(randomBytes))
}

func RandomBlockId() scTypes.BlockId {
	return scTypes.NewBlockId(RandomShardId(), RandomHash())
}

func RandomShardId() types.ShardId {
	for {
		randomBytes := make([]byte, 4)
		_, err := rand.Read(randomBytes)
		if err != nil {
			panic(err)
		}
		shardId := types.ShardId(binary.LittleEndian.Uint32(randomBytes))

		if shardId > types.MainShardId && shardId < types.InvalidShardId {
			return shardId
		}
	}
}

func NewRpcInTransaction() *jsonrpc.RPCInTransaction {
	return &jsonrpc.RPCInTransaction{
		Flags: types.NewTransactionFlags(types.TransactionFlagInternal, types.TransactionFlagRefund),
		Seqno: 10,
		From:  types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71"),
		To:    types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e72"),
		Value: types.NewValue(uint256.NewInt(1000)),
		Data:  []byte{10, 20, 30, 40},
	}
}

func NewBatchesSequence(batchesCount int) []*scTypes.BlockBatch {
	batches := make([]*scTypes.BlockBatch, 0, batchesCount)
	for range batchesCount {
		nextBatch := NewBlockBatch(ShardsCount)
		if len(batches) == 0 {
			nextBatch.MainShardBlock.Number = 0
			nextBatch.MainShardBlock.ParentHash = common.EmptyHash
		} else {
			prevBatch := batches[len(batches)-1]
			nextBatch.MainShardBlock.ParentHash = prevBatch.MainShardBlock.Hash
			nextBatch.MainShardBlock.Number = prevBatch.MainShardBlock.Number + 1
			nextBatch.ParentId = &prevBatch.Id
		}
		batches = append(batches, nextBatch)
	}
	return batches
}

func NewBlockBatch(childBlocksCount int) *scTypes.BlockBatch {
	mainBlock := NewMainShardBlock()
	childBlocks := make([]*jsonrpc.RPCBlock, 0, childBlocksCount)
	mainBlock.ChildBlocks = nil

	for i := range childBlocksCount {
		block := NewExecutionShardBlock()
		block.ShardId = types.ShardId(i + 1)
		childBlocks = append(childBlocks, block)
		mainBlock.ChildBlocks = append(mainBlock.ChildBlocks, block.Hash)
	}

	batch, err := scTypes.NewBlockBatch(nil, mainBlock, childBlocks)
	if err != nil {
		panic(err)
	}
	return batch
}

func NewMainShardBlock() *jsonrpc.RPCBlock {
	childHashes := make([]common.Hash, 0, ShardsCount)
	for range ShardsCount {
		childHashes = append(childHashes, RandomHash())
	}

	return &jsonrpc.RPCBlock{
		Number:              RandomBlockNum(),
		ShardId:             types.MainShardId,
		ChildBlocks:         childHashes,
		ChildBlocksRootHash: RandomHash(),
		Hash:                RandomHash(),
		ParentHash:          RandomHash(),
		Transactions:        newRpcInTransactions(ShardsCount),
	}
}

func NewExecutionShardBlock() *jsonrpc.RPCBlock {
	return &jsonrpc.RPCBlock{
		Number:        RandomBlockNum(),
		ShardId:       RandomShardId(),
		Hash:          RandomHash(),
		MainChainHash: RandomHash(),
		ParentHash:    RandomHash(),
		Transactions:  newRpcInTransactions(ShardsCount),
	}
}

func NewProposalData(txCount int, currentTime time.Time) *scTypes.ProposalData {
	transactions := make([]*scTypes.PrunedTransaction, 0, txCount)
	for range txCount {
		tx := scTypes.NewTransaction(NewRpcInTransaction())
		transactions = append(transactions, tx)
	}

	return &scTypes.ProposalData{
		MainShardBlockHash: RandomHash(),
		Transactions:       transactions,
		OldProvedStateRoot: RandomHash(),
		NewProvedStateRoot: RandomHash(),
		MainBlockFetchedAt: currentTime.Add(-time.Hour),
	}
}

func newRpcInTransactions(count int) []*jsonrpc.RPCInTransaction {
	transactions := make([]*jsonrpc.RPCInTransaction, count)
	for i := range count {
		transactions[i] = NewRpcInTransaction()
	}
	return transactions
}
