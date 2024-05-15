package db

import (
	"fmt"
	"github.com/NilFoundation/nil/core/types"
	"strconv"
)

const (
	blockTable = "Blocks"

	codeTable = "Code"

	contractTrieTable    = "ContractTrie"
	storageTrieTable     = "StorageTrie"
	shardBlocksTrieTable = "ShardBlocksTrie"
	messageTrieTable     = "MessageTrie"

	contractTable = "Contract"
	messageTable  = "Message"

	LastBlockTable = "LastBlock"
)

func tableName(tableName string, shardId types.ShardId) string {
	return fmt.Sprintf("%s:%d:", tableName, shardId)
}

func ContractTrieTableName(shardId types.ShardId) string {
	return tableName(contractTrieTable, shardId)
}

func MessageTrieTableName(shardId types.ShardId) string {
	return tableName(messageTrieTable, shardId)
}

func StorageTrieTableName(shardId types.ShardId) string {
	return tableName(storageTrieTable, shardId)
}

func ShardBlocksTrieTableName(blockId uint64) string {
	return shardBlocksTrieTable + strconv.FormatUint(blockId, 10)
}
