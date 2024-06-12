package db

import (
	"errors"
	"reflect"

	fastssz "github.com/NilFoundation/fastssz"
	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
)

// todo: return errors
func readDecodable[
	S any,
	T interface {
		~*S
		fastssz.Unmarshaler
	},
](tx RoTx, table ShardedTableName, shardId types.ShardId, hash common.Hash) (*S, error) {
	data, err := tx.GetFromShard(shardId, table, hash.Bytes())
	if err != nil {
		return nil, err
	}

	decoded := new(S)
	if err := T(decoded).UnmarshalSSZ(data); err != nil {
		return nil, err
	}
	return decoded, nil
}

func writeRawKeyEncodable[
	T interface {
		fastssz.Marshaler
	},
](tx RwTx, tableName ShardedTableName, shardId types.ShardId, key []byte, value T) error {
	data, err := value.MarshalSSZ()
	if err != nil {
		return err
	}

	return tx.PutToShard(shardId, tableName, key, data)
}

func writeEncodable[
	T interface {
		fastssz.Marshaler
		common.Hashable
	},
](tx RwTx, tableName ShardedTableName, shardId types.ShardId, obj T) error {
	return writeRawKeyEncodable(tx, tableName, shardId, obj.Hash().Bytes(), obj)
}

func ReadVersionInfo(tx RoTx) (*types.VersionInfo, error) {
	rawVersionInfo, err := tx.Get(SchemeVersionTable, []byte(types.SchemeVersionInfoKey))
	if err != nil {
		return nil, err
	}
	res := &types.VersionInfo{}
	if err := res.UnmarshalSSZ(rawVersionInfo); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteVersionInfo(tx RwTx, version *types.VersionInfo) error {
	rawVersionInfo, err := version.MarshalSSZ()
	if err != nil {
		return err
	}
	return tx.Put(SchemeVersionTable, []byte(types.SchemeVersionInfoKey), rawVersionInfo)
}

func IsVersionOutdated(tx RoTx) (bool, error) {
	dbVersion, err := ReadVersionInfo(tx)
	if errors.Is(err, ErrKeyNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !reflect.DeepEqual(dbVersion, types.NewVersionInfo()), nil
}

func ReadBlock(tx RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, error) {
	return readDecodable[types.Block, *types.Block](tx, blockTable, shardId, hash)
}

func ReadLastBlock(tx RoTx, shardId types.ShardId) (*types.Block, error) {
	hash, err := ReadLastBlockHash(tx, shardId)
	if err != nil {
		return nil, err
	}
	return readDecodable[types.Block, *types.Block](tx, blockTable, shardId, hash)
}

func ReadCollatorState(tx RoTx, shardId types.ShardId) (types.CollatorState, error) {
	res := types.CollatorState{}
	buf, err := tx.Get(CollatorStateTable, shardId.Bytes())
	if err != nil {
		return res, err
	}

	if err := res.UnmarshalSSZ(buf); err != nil {
		return res, err
	}
	return res, nil
}

func WriteCollatorState(tx RwTx, shardId types.ShardId, state types.CollatorState) error {
	value, err := state.MarshalSSZ()
	if err != nil {
		return err
	}
	return tx.Put(CollatorStateTable, shardId.Bytes(), value)
}

func ReadLastBlockHash(tx RoTx, shardId types.ShardId) (common.Hash, error) {
	h, err := tx.Get(LastBlockTable, shardId.Bytes())
	return common.BytesToHash(h), err
}

func WriteLastBlockHash(tx RwTx, shardId types.ShardId, hash common.Hash) error {
	return tx.Put(LastBlockTable, shardId.Bytes(), hash.Bytes())
}

func WriteBlock(tx RwTx, shardId types.ShardId, block *types.Block) error {
	return writeEncodable(tx, blockTable, shardId, block)
}

func ReadContract(tx RoTx, shardId types.ShardId, hash common.Hash) (*types.SmartContract, error) {
	return readDecodable[types.SmartContract, *types.SmartContract](tx, contractTable, shardId, hash)
}

func WriteContract(tx RwTx, shardId types.ShardId, contract *types.SmartContract) error {
	return writeEncodable(tx, contractTable, shardId, contract)
}

func WriteCode(tx RwTx, shardId types.ShardId, code types.Code) error {
	return tx.PutToShard(shardId, codeTable, code.Hash().Bytes(), code[:])
}

func ReadCode(tx RoTx, shardId types.ShardId, hash common.Hash) (types.Code, error) {
	return tx.GetFromShard(shardId, codeTable, hash.Bytes())
}

func ReadBlockHashByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (common.Hash, error) {
	blockHash, err := tx.GetFromShard(shardId, BlockHashByNumberIndex, blockNumber.Bytes())
	return common.BytesToHash(blockHash), err
}

func ReadBlockByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (*types.Block, error) {
	blockHash, err := ReadBlockHashByNumber(tx, shardId, blockNumber)
	if err != nil {
		return nil, err
	}
	return ReadBlock(tx, shardId, blockHash)
}
