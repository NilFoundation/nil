package db

import (
	"errors"

	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	fastssz "github.com/ferranbt/fastssz"
)

func readDecodable[
	S any,
	T interface {
		~*S
		fastssz.Unmarshaler
	},
](tx Tx, table ShardedTableName, shardId types.ShardId, hash common.Hash) *S {
	data, err := tx.GetFromShard(shardId, table, hash.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		logger.Fatal().Msgf("Read from table %s [%s] failed. err: %s", table, shardId, err.Error())
	}
	if data == nil {
		return nil
	}
	decoded := new(S)
	if err := T(decoded).UnmarshalSSZ(*data); err != nil {
		logger.Fatal().Msgf("Invalid SSZ while reading from %s. hash: %v, err: %v", table, hash, err)
	}
	return decoded
}

func writeEncodable[
	T interface {
		fastssz.Marshaler
		common.Hashable
	},
](tx Tx, tableName ShardedTableName, shardId types.ShardId, obj T) error {
	hash := obj.Hash()

	data, err := obj.MarshalSSZ()
	if err != nil {
		return err
	}

	return tx.PutToShard(shardId, tableName, hash.Bytes(), data)
}

func ReadCurrentVersion() *types.VersionInfo {
	x1 := new(types.SmartContract).Hash()
	x2 := new(types.Block).Hash()
	x3 := new(types.Message).Hash()
	x1b := append(append(x1.Bytes(), x2.Bytes()...), x3.Bytes()...)
	return &types.VersionInfo{Version: common.PoseidonHash(x1b)}
}

func ReadDbVersion(tx Tx) (*types.VersionInfo, error) {
	rawVersionInfo, err := tx.Get(DatabaseInfoTable, []byte("VersionInfo"))
	if err != nil {
		return nil, err
	}
	res := types.VersionInfo{}
	err = res.UnmarshalSSZ(*rawVersionInfo)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func WriteDbVersion(version *types.VersionInfo, tx Tx) error {
	rawVersionInfo, err := version.MarshalSSZ()
	if err != nil {
		return err
	}
	err = tx.Put(DatabaseInfoTable, []byte("VersionInfo"), rawVersionInfo)
	return err
}

func CompareVersion(tx Tx) (bool, error) {
	v1, err := ReadDbVersion(tx)
	if errors.Is(err, ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	v2 := ReadCurrentVersion()
	return v1.Version == v2.Version, nil
}

func ReadBlock(tx Tx, shardId types.ShardId, hash common.Hash) *types.Block {
	return readDecodable[types.Block, *types.Block](tx, blockTable, shardId, hash)
}

func WriteBlock(tx Tx, shardId types.ShardId, block *types.Block) error {
	return writeEncodable(tx, blockTable, shardId, block)
}

func ReadContract(tx Tx, shardId types.ShardId, hash common.Hash) *types.SmartContract {
	return readDecodable[types.SmartContract, *types.SmartContract](tx, contractTable, shardId, hash)
}

func WriteContract(tx Tx, shardId types.ShardId, contract *types.SmartContract) error {
	return writeEncodable(tx, contractTable, shardId, contract)
}

func WriteCode(tx Tx, shardId types.ShardId, code types.Code) error {
	hash := code.Hash()
	if err := tx.PutToShard(shardId, codeTable, hash.Bytes(), code[:]); err != nil {
		return err
	}
	return nil
}

func ReadCode(tx Tx, shardId types.ShardId, hash common.Hash) (types.Code, error) {
	code, err := tx.GetFromShard(shardId, codeTable, hash[:])
	if errors.Is(err, ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, nil
	}

	res := types.Code(*code)
	return res, nil
}

// TODO: Use hash -> (blockNumber, txIndex) mapping and message trie instead of duplicating messages.
func ReadMessage(tx Tx, shardId types.ShardId, hash common.Hash) *types.Message {
	return readDecodable[types.Message, *types.Message](tx, messageTable, shardId, hash)
}

func WriteMessage(tx Tx, shardId types.ShardId, message *types.Message) error {
	return writeEncodable(tx, messageTable, shardId, message)
}
