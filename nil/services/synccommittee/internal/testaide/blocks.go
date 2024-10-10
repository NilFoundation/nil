//go:build test

package testaide

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
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
	return types.BlockNumber(binary.BigEndian.Uint64(randomBytes))
}

func GenerateMainShardBlock() *jsonrpc.RPCBlock {
	return &jsonrpc.RPCBlock{Number: RandomBlockNum(), ShardId: types.MainShardId, Hash: RandomHash()}
}
