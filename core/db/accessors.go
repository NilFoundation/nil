package db

import (
	"errors"
	"reflect"

	fastssz "github.com/NilFoundation/fastssz"
	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog/log"
)

// todo: return errors
func readDecodable[
	S any,
	T interface {
		~*S
		fastssz.Unmarshaler
	},
](tx RoTx, table ShardedTableName, shardId types.ShardId, hash common.Hash) *S {
	data, err := tx.GetFromShard(shardId, table, hash.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return nil
	}
	check.LogAndPanicIfErrf(err, log.Logger, "Read from table %s [%s] failed.", table, shardId)
	if data == nil {
		return nil
	}

	decoded := new(S)
	err = T(decoded).UnmarshalSSZ(*data)
	check.LogAndPanicIfErrf(err, log.Logger, "Invalid SSZ while reading from %s. hash: %v", table, hash)

	return decoded
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
	res := types.VersionInfo{}
	err = res.UnmarshalSSZ(*rawVersionInfo)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func WriteVersionInfo(tx RwTx, version *types.VersionInfo) error {
	rawVersionInfo, err := version.MarshalSSZ()
	if err != nil {
		return err
	}
	err = tx.Put(SchemeVersionTable, []byte(types.SchemeVersionInfoKey), rawVersionInfo)
	return err
}

func IsVersionOutdated(tx RoTx) (bool, error) {
	dbVersion, err := ReadVersionInfo(tx)
	if errors.Is(err, ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !reflect.DeepEqual(dbVersion, types.NewVersionInfo()), nil
}

func ReadBlock(tx RoTx, shardId types.ShardId, hash common.Hash) *types.Block {
	return readDecodable[types.Block, *types.Block](tx, blockTable, shardId, hash)
}

func ReadLastBlock(tx RoTx, shardId types.ShardId) (*types.Block, error) {
	hash, err := ReadLastBlockHash(tx, shardId)
	if err != nil {
		return nil, err
	}
	return readDecodable[types.Block, *types.Block](tx, blockTable, shardId, hash), nil
}

func ReadCollatorState(tx RoTx, shardId types.ShardId) (types.CollatorState, error) {
	res := types.CollatorState{}
	buf, err := tx.Get(CollatorStateTable, shardId.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return res, nil
	}
	if err != nil {
		return res, err
	}

	if err = res.UnmarshalSSZ(*buf); err != nil {
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
	if errors.Is(err, ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}
	return common.BytesToHash(*h), nil
}

func WriteBlock(tx RwTx, shardId types.ShardId, block *types.Block) error {
	return writeEncodable(tx, blockTable, shardId, block)
}

func ReadContract(tx RoTx, shardId types.ShardId, hash common.Hash) *types.SmartContract {
	return readDecodable[types.SmartContract, *types.SmartContract](tx, contractTable, shardId, hash)
}

func WriteContract(tx RwTx, shardId types.ShardId, contract *types.SmartContract) error {
	return writeEncodable(tx, contractTable, shardId, contract)
}

func WriteCode(tx RwTx, shardId types.ShardId, code types.Code) error {
	hash := code.Hash()
	if err := tx.PutToShard(shardId, codeTable, hash.Bytes(), code[:]); err != nil {
		return err
	}
	return nil
}

func ReadCode(tx RoTx, shardId types.ShardId, hash common.Hash) (types.Code, error) {
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

func ReadBlockHashByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (common.Hash, error) {
	blockHash, err := tx.GetFromShard(shardId, BlockHashByNumberIndex, blockNumber.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}
	return common.BytesToHash(*blockHash), nil
}

func ReadBlockByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (*types.Block, error) {
	blockHash, err := ReadBlockHashByNumber(tx, shardId, blockNumber)
	if err != nil {
		return nil, err
	}
	if blockHash == common.EmptyHash {
		return nil, nil
	}
	return ReadBlock(tx, shardId, blockHash), nil
}
