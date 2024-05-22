package db

import (
	"fmt"

	"github.com/NilFoundation/nil/core/types"
)

type TableName string

type ShardedTableName string

const (
	blockTable = ShardedTableName("Blocks")

	codeTable = ShardedTableName("Code")

	ContractTrieTable    = ShardedTableName("ContractTrie")
	StorageTrieTable     = ShardedTableName("StorageTrie")
	shardBlocksTrieTable = ShardedTableName("ShardBlocksTrie")
	MessageTrieTable     = ShardedTableName("MessageTrie")
	ReceiptTrieTable     = ShardedTableName("ReceiptTrie")

	contractTable = ShardedTableName("Contract")
	messageTable  = ShardedTableName("Message")

	LastBlockTable = TableName("LastBlock")

	DatabaseInfoTable = TableName("DatabaseInfo")
)

func shardTableName(tableName ShardedTableName, shardId types.ShardId) TableName {
	return TableName(fmt.Sprintf("%s:%s", tableName, shardId))
}

func ShardBlocksTrieTableName(blockId uint64) ShardedTableName {
	return ShardedTableName(fmt.Sprintf("%s%d", shardBlocksTrieTable, blockId))
}
