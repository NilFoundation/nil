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

	for _, subgraph := range NewSubgraphSequence(batchesCount) {
		var parentId *scTypes.BatchId
		if len(batches) > 0 {
			parentId = &batches[len(batches)-1].Id
		}

		batch, err := scTypes.NewBlockBatch(parentId).WithAddedSubgraph(subgraph)
		check.PanicIfErr(err)
		batches = append(batches, batch)
	}

	return batches
}

func NewSubgraphSequence(subgraphsCount int) []scTypes.Subgraph {
	if subgraphsCount <= 0 {
		log.Panicf("subgraphsCount must be positive, got=%d", subgraphsCount)
	}

	firstSubgraph := NewSubgraph(ShardsCount)
	firstSubgraph.Main.Number = 1
	for _, segment := range firstSubgraph.Children {
		segment[0].Number = 1
		segment[0].ParentHash = GenesisBlockHash
	}

	subgraphs := make([]scTypes.Subgraph, 1, subgraphsCount)
	subgraphs[0] = firstSubgraph

	for range subgraphsCount - 1 {
		nextSubgraph := NewSubgraph(ShardsCount)
		prevSubgraph := subgraphs[len(subgraphs)-1]

		nextSubgraph.Main.ParentHash = prevSubgraph.Main.Hash
		nextSubgraph.Main.Number = prevSubgraph.Main.Number + 1

		for shard, segment := range nextSubgraph.Children {
			prevSegmentTail := prevSubgraph.Children[shard].Latest()
			segment[0].ParentHash = prevSegmentTail.Hash
			segment[0].Number = prevSegmentTail.Number + 1
		}

		subgraphs = append(subgraphs, nextSubgraph)
	}
	return subgraphs
}

func NewBlockBatch(shardCount int) *scTypes.BlockBatch {
	subgraph := NewSubgraph(shardCount)
	batch, err := scTypes.NewBlockBatch(nil).WithAddedSubgraph(subgraph)
	check.PanicIfErr(err)
	return batch
}

func NewSubgraph(shardCount int) scTypes.Subgraph {
	mainBlock := NewMainShardBlock()
	mainBlock.ChildBlocks = nil

	children := make(map[types.ShardId]scTypes.ShardChainSegment)
	for i := range shardCount {
		block := NewExecutionShardBlock()
		block.ShardId = types.ShardId(i + 1)
		mainBlock.ChildBlocks = append(mainBlock.ChildBlocks, block.Hash)
		children[block.ShardId] = []*scTypes.Block{block}
	}

	subgraph, err := scTypes.NewSubgraph(mainBlock, children)
	check.PanicIfErr(err)
	return *subgraph
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
