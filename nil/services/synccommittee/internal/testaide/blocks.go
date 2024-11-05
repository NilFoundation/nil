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

func GenerateRpcInMessage() *jsonrpc.RPCInMessage {
	return &jsonrpc.RPCInMessage{
		Flags: types.NewMessageFlags(types.MessageFlagInternal, types.MessageFlagRefund),
		Seqno: 10,
		From:  types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71"),
		To:    types.HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e72"),
		Value: types.NewValue(uint256.NewInt(1000)),
		Data:  []byte{10, 20, 30, 40},
	}
}

func GenerateMainShardBlocks(blocksCount int) []*jsonrpc.RPCBlock {
	mainShardBlocks := make([]*jsonrpc.RPCBlock, 0, blocksCount)
	for range blocksCount {
		nextBlock := GenerateMainShardBlock()
		if len(mainShardBlocks) > 0 {
			prevBlock := mainShardBlocks[len(mainShardBlocks)-1]
			nextBlock.ParentHash = prevBlock.Hash
			nextBlock.Number = prevBlock.Number + 1
		}
		mainShardBlocks = append(mainShardBlocks, nextBlock)
	}
	return mainShardBlocks
}

func GenerateBlockBatch(childBlocksCount int) (mainBlock *jsonrpc.RPCBlock, childBlocks []*jsonrpc.RPCBlock) {
	mainBlock = GenerateMainShardBlock()
	childBlocks = make([]*jsonrpc.RPCBlock, 0, childBlocksCount)

	for i := range childBlocksCount {
		block := GenerateExecutionShardBlock()
		block.ShardId = types.ShardId(i + 1)
		childBlocks = append(childBlocks, block)
		mainBlock.ChildBlocks = append(mainBlock.ChildBlocks, block.Hash)
	}

	return mainBlock, childBlocks
}

func GenerateMainShardBlock() *jsonrpc.RPCBlock {
	return &jsonrpc.RPCBlock{
		Number:              RandomBlockNum(),
		ShardId:             types.MainShardId,
		ChildBlocksRootHash: RandomHash(),
		Hash:                RandomHash(),
		ParentHash:          RandomHash(),
		Messages:            generateRpcInMessages(5),
	}
}

func GenerateExecutionShardBlocks(blocksCount int) []*jsonrpc.RPCBlock {
	executionShardBlocks := make([]*jsonrpc.RPCBlock, 0, blocksCount)
	for i := range blocksCount {
		block := GenerateExecutionShardBlock()
		block.ShardId = types.ShardId(i + 1)
		executionShardBlocks = append(executionShardBlocks, block)
	}
	return executionShardBlocks
}

func GenerateExecutionShardBlock() *jsonrpc.RPCBlock {
	return &jsonrpc.RPCBlock{
		Number:        RandomBlockNum(),
		ShardId:       RandomShardId(),
		Hash:          RandomHash(),
		MainChainHash: RandomHash(),
		ParentHash:    RandomHash(),
		Messages:      generateRpcInMessages(5),
	}
}

func GenerateProposalData(txCount int) *scTypes.ProposalData {
	transactions := make([]scTypes.PrunedTransaction, 0, txCount)
	for range txCount {
		tx := scTypes.NewTransaction(GenerateRpcInMessage())
		transactions = append(transactions, tx)
	}

	return &scTypes.ProposalData{
		MainShardBlockHash: RandomHash(),
		Transactions:       transactions,
		OldProvedStateRoot: RandomHash(),
		NewProvedStateRoot: RandomHash(),
		MainBlockFetchedAt: time.Now().Add(-time.Hour),
	}
}

func generateRpcInMessages(count int) []*jsonrpc.RPCInMessage {
	messages := make([]*jsonrpc.RPCInMessage, count)
	for i := range count {
		messages[i] = GenerateRpcInMessage()
	}
	return messages
}
