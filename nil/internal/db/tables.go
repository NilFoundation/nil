package db

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type TableName string

type ShardedTableName string

const (
	blockTable           = ShardedTableName("Blocks")
	blockTimestampTable  = ShardedTableName("BlockTimestamp")
	codeTable            = ShardedTableName("Code")
	shardBlocksTrieTable = ShardedTableName("ShardBlocksTrie")

	ContractTrieTable                               = ShardedTableName("ContractTrie")
	StorageTrieTable                                = ShardedTableName("StorageTrie")
	TransactionTrieTable                            = ShardedTableName("TransactionTrie")
	ReceiptTrieTable                                = ShardedTableName("ReceiptTrie")
	TokenTrieTable                                  = ShardedTableName("TokenTrie")
	ConfigTrieTable                                 = ShardedTableName("ConfigTrie")
	BlockHashByNumberIndex                          = ShardedTableName("BlockHashByNumber")
	BlockHashAndInTransactionIndexByTransactionHash = ShardedTableName(
		"BlockHashAndInTransactionIndexByTransactionHash")
	BlockHashAndOutTransactionIndexByTransactionHash = ShardedTableName(
		"BlockHashAndOutTransactionIndexByTransactionHash")
	AsyncCallContextTable = ShardedTableName("AsyncCallContext")

	collatorStateTable          = TableName("CollatorState")
	errorByTransactionHashTable = TableName("ErrorByTransactionHash")
	schemeVersionTable          = TableName("SchemeVersion")
	LastBlockTable              = TableName("LastBlock")

	DHTTable = TableName("DHT")
)

func ShardTableName(tableName ShardedTableName, shardId types.ShardId) TableName {
	return TableName(fmt.Sprintf("%s:%s", tableName, shardId))
}

func ShardBlocksTrieTableName(blockId types.BlockNumber) ShardedTableName {
	return ShardedTableName(fmt.Sprintf("%s%d", shardBlocksTrieTable, blockId))
}

type BlockHashAndTransactionIndex struct {
	BlockHash        common.Hash
	TransactionIndex types.TransactionIndex
}

func (i *BlockHashAndTransactionIndex) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, i)
}

func (i BlockHashAndTransactionIndex) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&i)
}
