package db

import (
	"fmt"
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

func tableName(tableName string, shardId int) string {
	return fmt.Sprintf("%s:%d:", tableName, shardId)
}

func ContractTrieTableName(shardId int) string {
	return tableName(contractTrieTable, shardId)
}

func MessageTrieTableName(shardId int) string {
	return tableName(messageTrieTable, shardId)
}

func StorageTrieTableName(shardId int) string {
	return tableName(storageTrieTable, shardId)
}

func ShardBlocksTrieTableName(blockId uint64) string {
	return shardBlocksTrieTable + strconv.FormatUint(blockId, 10)
}
