package db

import (
	"errors"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
	types "github.com/NilFoundation/nil/core/types"
)

func readDecodable[
	S any,
	T interface {
		~*S
		ssz.SSZDecodable
	},
](tx Tx, table string, shardId int, hash common.Hash) *S {
	data, err := tx.Get(tableName(table, shardId), hash.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		logger.Fatal().Msgf("Read from table %s failed. err: %s", table, err.Error())
	}
	if data == nil {
		return nil
	}
	decoded := new(S)
	if err := T(decoded).DecodeSSZ(*data, 0); err != nil {
		logger.Fatal().Msgf("Invalid RLP while reading from %s. hash: %v, err: %v", table, hash, err)
	}
	return decoded
}

func writeEncodable[
	T interface {
		ssz.SSZEncodable
		common.Hashable
	},
](tx Tx, table string, shardId int, obj T) error {
	hash := obj.Hash()

	data, err := obj.EncodeSSZ(nil)
	if err != nil {
		return err
	}

	return tx.Put(tableName(table, shardId), hash.Bytes(), data)
}

/*
TODO: eventually, ReadBlock and WriteBlock should accept the shardId
parameter. Currently, however, the RPC doesn't contain shardId parameters
for fetching shard by hash, and it would take time to do the shardId resolution
correctly for that case.
*/
func ReadBlock(tx Tx, hash common.Hash) *types.Block {
	return readDecodable[types.Block, *types.Block](tx, blockTable, 0, hash)
}

func WriteBlock(tx Tx, block *types.Block) error {
	return writeEncodable(tx, blockTable, 0, block)
}

func ReadContract(tx Tx, shardId int, hash common.Hash) *types.SmartContract {
	return readDecodable[types.SmartContract, *types.SmartContract](tx, contractTable, shardId, hash)
}

func WriteContract(tx Tx, shardId int, contract *types.SmartContract) error {
	return writeEncodable(tx, contractTable, shardId, contract)
}

func WriteCode(tx Tx, shardId int, code types.Code) error {
	hash := code.Hash()
	if err := tx.Put(tableName(codeTable, shardId), hash.Bytes(), code[:]); err != nil {
		return err
	}
	return nil
}

func ReadCode(tx Tx, shardId int, hash common.Hash) (*types.Code, error) {
	code, err := tx.Get(tableName(codeTable, shardId), hash[:])
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, nil
	}

	res := types.Code(*code)
	return &res, nil
}

// TODO: Use hash -> (blockNumber, txIndex) mapping and message trie instead of duplicating messages.
func ReadMessage(tx Tx, shardId int, hash common.Hash) *types.Message {
	return readDecodable[types.Message, *types.Message](tx, messageTable, shardId, hash)
}

func WriteMessage(tx Tx, shardId int, message *types.Message) error {
	return writeEncodable(tx, messageTable, shardId, message)
}
