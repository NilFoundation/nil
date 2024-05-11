package db

import "fmt"

const (
	BlockTable = "Blocks"

	CodeTable         = "Code"
	ContractCodeTable = "ContractCode"

	ContractTrieTable = "ContractTrie"
	StorageTrieTable  = "StorageTrie"

	ContractTable = "Contract"
	StorageTable  = "Storage"

	MptTable = "MPT"

	LastBlockTable = "LastBlock"
)

func TableName(tableName string, shardId int) string {
	return fmt.Sprintf("%s:%d:", tableName, shardId)
}
