package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type prettyKey interface {
	fmt.Stringer
	Bytes() []byte
}

func Get(tx RoTx, table TableName, key prettyKey) ([]byte, error) {
	data, err := tx.Get(table, key.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return nil, fmt.Errorf("%w: table=%s, key=%s", err, table, key)
	}
	return data, err
}

func GetFromShard(tx RoTx, shardId types.ShardId, table ShardedTableName, key prettyKey) ([]byte, error) {
	data, err := tx.GetFromShard(shardId, table, key.Bytes())
	if errors.Is(err, ErrKeyNotFound) {
		return nil, fmt.Errorf("%w: shard=%d, table=%s, key=%s", err, shardId, table, key)
	}
	return data, err
}

func readDecodable[
	T interface {
		~*S
		serialization.NilUnmarshaler
	},
	S any,
](tx RoTx, table ShardedTableName, shardId types.ShardId, hash common.Hash) (*S, error) {
	data, err := GetFromShard(tx, shardId, table, hash)
	if err != nil {
		return nil, err
	}

	decoded := new(S)
	if err := T(decoded).UnmarshalNil(data); err != nil {
		return nil, err
	}
	return decoded, nil
}

func writeEncodable[T serialization.NilMarshaler](
	tx RwTx, tableName ShardedTableName, shardId types.ShardId, hash common.Hash, obj T,
) error {
	data, err := obj.MarshalNil()
	if err != nil {
		return err
	}

	return tx.PutToShard(shardId, tableName, hash.Bytes(), data)
}

func ReadVersionInfo(tx RoTx) (*types.VersionInfo, error) {
	rawVersionInfo, err := tx.Get(schemeVersionTable, []byte(types.SchemeVersionInfoKey))
	if err != nil {
		return nil, err
	}
	res := &types.VersionInfo{}
	if err := res.UnmarshalNil(rawVersionInfo); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteVersionInfo(tx RwTx, version *types.VersionInfo) error {
	rawVersionInfo, err := version.MarshalNil()
	if err != nil {
		return err
	}
	return tx.Put(schemeVersionTable, []byte(types.SchemeVersionInfoKey), rawVersionInfo)
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

func ReadBlock(tx RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, error) {
	return readDecodable[*types.Block](tx, blockTable, shardId, hash)
}

func ReadBlockBytes(tx RoTx, shardId types.ShardId, hash common.Hash) ([]byte, error) {
	return GetFromShard(tx, shardId, blockTable, hash)
}

func ReadLastBlock(tx RoTx, shardId types.ShardId) (*types.Block, common.Hash, error) {
	hash, err := ReadLastBlockHash(tx, shardId)
	if err != nil {
		return nil, common.EmptyHash, err
	}
	b, err := readDecodable[*types.Block](tx, blockTable, shardId, hash)
	if err != nil {
		return nil, common.EmptyHash, err
	}
	return b, hash, nil
}

func ReadCollatorState(tx RoTx, shardId types.ShardId) (types.CollatorState, error) {
	res := types.CollatorState{}
	buf, err := Get(tx, collatorStateTable, shardId)
	if err != nil {
		return res, err
	}

	if err := res.UnmarshalNil(buf); err != nil {
		return res, err
	}
	return res, nil
}

func WriteCollatorState(tx RwTx, shardId types.ShardId, state types.CollatorState) error {
	value, err := state.MarshalNil()
	if err != nil {
		return err
	}
	return tx.Put(collatorStateTable, shardId.Bytes(), value)
}

func ReadLastBlockHash(tx RoTx, shardId types.ShardId) (common.Hash, error) {
	h, err := Get(tx, LastBlockTable, shardId)
	return common.BytesToHash(h), err
}

func WriteLastBlockHash(tx RwTx, shardId types.ShardId, hash common.Hash) error {
	return tx.Put(LastBlockTable, shardId.Bytes(), hash.Bytes())
}

func WriteBlockTimestamp(tx RwTx, shardId types.ShardId, blockHash common.Hash, timestamp uint64) error {
	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, timestamp)
	return tx.PutToShard(shardId, blockTimestampTable, blockHash.Bytes(), value)
}

func ReadBlockTimestamp(tx RoTx, shardId types.ShardId, blockHash common.Hash) (uint64, error) {
	value, err := GetFromShard(tx, shardId, blockTimestampTable, blockHash)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value), nil
}

func WriteBlock(tx RwTx, shardId types.ShardId, hash common.Hash, block *types.Block) error {
	return writeEncodable(tx, blockTable, shardId, hash, block)
}

func WriteError(tx RwTx, txnHash common.Hash, errMsg string) error {
	return tx.Put(errorByTransactionHashTable, txnHash.Bytes(), []byte(errMsg))
}

func ReadError(tx RoTx, txnHash common.Hash) (string, error) {
	res, err := Get(tx, errorByTransactionHashTable, txnHash)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func WriteCode(tx RwTx, shardId types.ShardId, hash common.Hash, code types.Code) error {
	if len(code) == 0 {
		if assert.Enable {
			check.PanicIfNot(hash == types.EmptyCodeHash)
		}

		return nil
	}

	return tx.PutToShard(shardId, codeTable, hash.Bytes(), code[:])
}

func ReadCode(tx RoTx, shardId types.ShardId, hash common.Hash) (types.Code, error) {
	if assert.Enable {
		check.PanicIfNot(!hash.Empty())
	}

	if hash == types.EmptyCodeHash {
		return types.Code{}, nil
	}
	return GetFromShard(tx, shardId, codeTable, hash)
}

func ReadBlockHashByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (common.Hash, error) {
	blockHash, err := GetFromShard(tx, shardId, BlockHashByNumberIndex, blockNumber)
	return common.BytesToHash(blockHash), err
}

func ReadBlockByNumber(tx RoTx, shardId types.ShardId, blockNumber types.BlockNumber) (*types.Block, error) {
	blockHash, err := ReadBlockHashByNumber(tx, shardId, blockNumber)
	if err != nil {
		return nil, err
	}
	return ReadBlock(tx, shardId, blockHash)
}

func ReadTxnNumberByHash(
	tx RoTx, shardId types.ShardId, hash common.Hash,
) (BlockHashAndTransactionIndex, error) {
	var idx BlockHashAndTransactionIndex

	value, err := GetFromShard(tx, shardId, BlockHashAndInTransactionIndexByTransactionHash, hash)
	if err != nil {
		return idx, err
	}

	if err := idx.UnmarshalNil(value); err != nil {
		return idx, err
	}
	return idx, nil
}
