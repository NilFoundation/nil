package db

import (
	"bytes"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type TableName string

type ShardedTableName string

const (
	blockTable           = ShardedTableName("Blocks")
	blockTimestampTable  = ShardedTableName("BlockTimestamp")
	codeTable            = ShardedTableName("Code")
	shardBlocksTrieTable = ShardedTableName("ShardBlocksTrie")

	ContractTrieTable                        = ShardedTableName("ContractTrie")
	StorageTrieTable                         = ShardedTableName("StorageTrie")
	MessageTrieTable                         = ShardedTableName("MessageTrie")
	ReceiptTrieTable                         = ShardedTableName("ReceiptTrie")
	CurrencyTrieTable                        = ShardedTableName("CurrencyTrie")
	ConfigTrieTable                          = ShardedTableName("ConfigTrie")
	ContractTable                            = ShardedTableName("Contract")
	BlockHashByNumberIndex                   = ShardedTableName("BlockHashByNumber")
	BlockHashAndInMessageIndexByMessageHash  = ShardedTableName("BlockHashAndInMessageIndexByMessageHash")
	BlockHashAndOutMessageIndexByMessageHash = ShardedTableName("BlockHashAndOutMessageIndexByMessageHash")
	AsyncCallContextTable                    = ShardedTableName("AsyncCallContext")

	collatorStateTable      = TableName("CollatorState")
	errorByMessageHashTable = TableName("ErrorByMessageHash")
	schemeVersionTable      = TableName("SchemeVersion")
	LastBlockTable          = TableName("LastBlock")
)

func ShardTableName(tableName ShardedTableName, shardId types.ShardId) TableName {
	return TableName(fmt.Sprintf("%s:%s", tableName, shardId))
}

func ShardBlocksTrieTableName(blockId types.BlockNumber) ShardedTableName {
	return ShardedTableName(fmt.Sprintf("%s%d", shardBlocksTrieTable, blockId))
}

func IsKeyFromShardBlocksTrieTable(key []byte, shardId types.ShardId) bool {
	return shardId.IsMainShard() && bytes.HasPrefix(key, []byte(shardBlocksTrieTable))
}

func CreateKeyFromShardTableChecker(shardId types.ShardId) func([]byte) bool {
	shardTableNames := []ShardedTableName{
		blockTable,
		blockTimestampTable,
		codeTable,

		ContractTrieTable,
		StorageTrieTable,
		MessageTrieTable,
		ReceiptTrieTable,
		CurrencyTrieTable,
		ConfigTrieTable,
		ContractTable,
		BlockHashByNumberIndex,
		BlockHashAndInMessageIndexByMessageHash,
		BlockHashAndOutMessageIndexByMessageHash,
		AsyncCallContextTable,
	}

	shardTables := make([]TableName, len(shardTableNames))
	for i, t := range shardTableNames {
		shardTables[i] = ShardTableName(t, shardId)
	}

	systemTables := []TableName{
		LastBlockTable,
		collatorStateTable,
	}

	systemKeys := make(map[string]struct{})
	for _, t := range systemTables {
		k := MakeKey(t, shardId.Bytes())
		systemKeys[string(k)] = struct{}{}
	}

	return func(key []byte) bool {
		if _, exists := systemKeys[string(key)]; exists {
			return true
		}
		if IsKeyFromShardBlocksTrieTable(key, shardId) {
			return true
		}
		for _, shardedTable := range shardTables {
			if IsKeyFromTable(shardedTable, key) {
				return true
			}
		}
		return false
	}
}

type BlockHashAndMessageIndex struct {
	BlockHash    common.Hash
	MessageIndex types.MessageIndex
}
