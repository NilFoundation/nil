package db

import "fmt"

const (
	BlockTable = "Blocks"

	CodeTable         = "Code"
	ContractCodeTable = "ContractCode"

	ContractTrieTable    = "ContractTrie"
	StorageTrieTable     = "StorageTrie"
	ShardBlocksTrieTable = "ShardBlocksTrie"
	MessageTrieTable     = "MessageTrie"

	ContractTable = "Contract"
	StorageTable  = "Storage"
	MessageTable  = "Message"

	MptTable = "MPT"

	LastBlockTable = "LastBlock"
)

func TableName(tableName string, shardId int) string {
	return fmt.Sprintf("%s:%d:", tableName, shardId)
}
