package db

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
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
	CurrencyTrieTable    = ShardedTableName("CurrencyTrie")
	ConfigTrieTable      = ShardedTableName("ConfigTrie")

	contractTable = ShardedTableName("Contract")

	GasPerShardTable                         = TableName("GasPerShard")
	LastBlockTable                           = TableName("LastBlock")
	CollatorStateTable                       = TableName("CollatorState")
	BlockHashByNumberIndex                   = ShardedTableName("BlockHashByNumber")
	BlockHashAndInMessageIndexByMessageHash  = ShardedTableName("BlockHashAndInMessageIndexByMessageHash")
	BlockHashAndOutMessageIndexByMessageHash = ShardedTableName("BlockHashAndOutMessageIndexByMessageHash")
	BlockTimestampTable                      = ShardedTableName("BlockTimestamp")

	ErrorByMessageHashTable = TableName("ErrorByMessageHash")

	SchemeVersionTable = TableName("SchemeVersion")

	AsyncCallContextTable = ShardedTableName("AsyncCallContext")
)

func ShardTableName(tableName ShardedTableName, shardId types.ShardId) TableName {
	return TableName(fmt.Sprintf("%s:%s", tableName, shardId))
}

func ShardBlocksTrieTableName(blockId types.BlockNumber) ShardedTableName {
	return ShardedTableName(fmt.Sprintf("%s%d", shardBlocksTrieTable, blockId))
}

type BlockHashAndMessageIndex struct {
	BlockHash    common.Hash
	MessageIndex types.MessageIndex
}
