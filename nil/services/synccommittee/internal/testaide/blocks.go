//go:build test

package testaide

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/holiman/uint256"
)

const (
	ShardsCount = 4
)

var GenesisBlockHash = RandomHash()

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
		Transaction: jsonrpc.Transaction{
			Flags: types.NewTransactionFlags(types.TransactionFlagInternal, types.TransactionFlagRefund),
			Seqno: 10,
			From:  types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71"),
			To:    types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e72"),
			Value: types.NewValue(uint256.NewInt(1000)),
			Data:  []byte{10, 20, 30, 40},
		},
	}
}

func NewBatchesSequence(batchesCount int) []*scTypes.BlockBatch {
	if batchesCount <= 0 {
		log.Panicf("batchesCount must be positive, got=%d", batchesCount)
	}

	batches := make([]*scTypes.BlockBatch, 0, batchesCount)

	for _, subgraph := range NewSegmentsSequence(batchesCount) {
		var parentId *scTypes.BatchId
		if len(batches) > 0 {
			parentId = &batches[len(batches)-1].Id
		}

		batch, err := scTypes.NewBlockBatch(parentId).WithAddedBlocks(subgraph)
		check.PanicIfErr(err)
		batches = append(batches, batch)
	}

	return batches
}

func NewSegmentsSequence(count int) []scTypes.ChainSegments {
	if count <= 0 {
		log.Panicf("count must be positive, got=%d", count)
	}

	firstSegments := NewChainSegments(ShardsCount)

	for _, segment := range firstSegments {
		segment[0].Number = 1
		segment[0].ParentHash = GenesisBlockHash
	}

	segments := make([]scTypes.ChainSegments, 1, count)
	segments[0] = firstSegments

	for range count - 1 {
		nextSegments := NewChainSegments(ShardsCount)
		prevSegments := segments[len(segments)-1]

		for shard, segment := range nextSegments {
			prevSegmentTail := prevSegments[shard].Latest()
			segment[0].ParentHash = prevSegmentTail.Hash
			segment[0].Number = prevSegmentTail.Number + 1
		}

		segments = append(segments, nextSegments)
	}
	return segments
}

func NewBlockBatch(shardCount int) *scTypes.BlockBatch {
	segments := NewChainSegments(shardCount)
	batch, err := scTypes.NewBlockBatch(nil).WithAddedBlocks(segments)
	check.PanicIfErr(err)
	return batch
}

func NewChainSegments(shardCount int) scTypes.ChainSegments {
	mainBlock := NewMainShardBlock()
	mainBlock.ChildBlocks = nil

	blocks := make(map[types.ShardId][]*scTypes.Block, shardCount)

	for i := range shardCount {
		block := NewExecutionShardBlock()
		block.ShardId = types.ShardId(i + 1)
		mainBlock.ChildBlocks = append(mainBlock.ChildBlocks, block.Hash)
		blocks[block.ShardId] = []*scTypes.Block{block}
	}

	segments, err := scTypes.NewChainSegments(blocks)
	check.PanicIfErr(err)
	return segments
}

func NewMainShardBlock() *scTypes.Block {
	childHashes := make([]common.Hash, 0, ShardsCount)
	for range ShardsCount {
		childHashes = append(childHashes, RandomHash())
	}

	return &scTypes.Block{
		Number:              RandomBlockNum(),
		ShardId:             types.MainShardId,
		ChildBlocks:         childHashes,
		ChildBlocksRootHash: RandomHash(),
		Hash:                RandomHash(),
		ParentHash:          RandomHash(),
		Transactions:        newRpcInTransactions(ShardsCount),
	}
}

func NewExecutionShardBlock() *scTypes.Block {
	return &scTypes.Block{
		Number:        RandomBlockNum(),
		ShardId:       RandomShardId(),
		Hash:          RandomHash(),
		MainShardHash: RandomHash(),
		ParentHash:    RandomHash(),
		Transactions:  newRpcInTransactions(ShardsCount),
	}
}

func NewProposalData(currentTime time.Time) *scTypes.ProposalData {
	return scTypes.NewProposalData(
		scTypes.NewBatchId(),
		scTypes.DataProofs{},
		RandomHash(),
		RandomHash(),
		currentTime.Add(-time.Hour),
	)
}

func newRpcInTransactions(count int) []*jsonrpc.RPCInTransaction {
	transactions := make([]*jsonrpc.RPCInTransaction, count)
	for i := range count {
		transactions[i] = NewRpcInTransaction()
	}
	return transactions
}
