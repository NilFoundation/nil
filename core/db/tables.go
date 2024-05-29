package db

import (
	"fmt"

	"github.com/NilFoundation/nil/common"
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

	LastBlockTable                        = TableName("LastBlock")
	BlockHashByNumberIndex                = ShardedTableName("BlockHashByNumber")
	BlockHashAndMessageIndexByMessageHash = ShardedTableName("BlockHashAndMessageIndexByMessageHash")

	SchemeVersionTable = TableName("SchemeVersion")
)

func shardTableName(tableName ShardedTableName, shardId types.ShardId) TableName {
	return TableName(fmt.Sprintf("%s:%s", tableName, shardId))
}

func ShardBlocksTrieTableName(blockId types.BlockNumber) ShardedTableName {
	return ShardedTableName(fmt.Sprintf("%s%d", shardBlocksTrieTable, blockId))
}

type BlockHashAndMessageIndex struct {
	BlockHash    common.Hash
	MessageIndex types.MessageIndex
}
