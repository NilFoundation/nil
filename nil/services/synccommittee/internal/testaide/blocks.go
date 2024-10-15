//go:build test

package testaide

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
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

func GenerateExecutionShardBlock(mainShardBlockHash common.Hash) *jsonrpc.RPCBlock {
	return &jsonrpc.RPCBlock{
		Number:        RandomBlockNum(),
		ShardId:       RandomShardId(),
		Hash:          RandomHash(),
		MainChainHash: mainShardBlockHash,
		ParentHash:    RandomHash(),
		Messages:      generateRpcInMessages(5),
	}
}

func generateRpcInMessages(count int) []*jsonrpc.RPCInMessage {
	messages := make([]*jsonrpc.RPCInMessage, count)
	for i := range count {
		messages[i] = GenerateRpcInMessage()
	}
	return messages
}
