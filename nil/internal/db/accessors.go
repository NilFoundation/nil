package db

import (
	"encoding/binary"
	"errors"
	"reflect"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// todo: return errors
func readDecodable[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
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
	},
](tx RwTx, tableName ShardedTableName, shardId types.ShardId, hash common.Hash, obj T) error {
	return writeRawKeyEncodable(tx, tableName, shardId, hash.Bytes(), obj)
}

func ReadVersionInfo(tx RoTx) (*types.VersionInfo, error) {
	rawVersionInfo, err := tx.Get(schemeVersionTable, []byte(types.SchemeVersionInfoKey))
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

func ReadBlockSSZ(tx RoTx, shardId types.ShardId, hash common.Hash) ([]byte, error) {
	return tx.GetFromShard(shardId, blockTable, hash.Bytes())
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
	buf, err := tx.Get(collatorStateTable, shardId.Bytes())
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
	return tx.Put(collatorStateTable, shardId.Bytes(), value)
}

func ReadLastBlockHash(tx RoTx, shardId types.ShardId) (common.Hash, error) {
	h, err := tx.Get(LastBlockTable, shardId.Bytes())
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
	value, err := tx.GetFromShard(shardId, blockTimestampTable, blockHash.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value), nil
}

func WriteBlock(tx RwTx, shardId types.ShardId, hash common.Hash, block *types.Block) error {
	return writeEncodable(tx, blockTable, shardId, hash, block)
}

func WriteError(tx RwTx, msgHash common.Hash, errMsg string) error {
	return tx.Put(errorByMessageHashTable, msgHash.Bytes(), []byte(errMsg))
}

func ReadError(tx RoTx, msgHash common.Hash) (string, error) {
	res, err := tx.Get(errorByMessageHashTable, msgHash.Bytes())
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func WriteCode(tx RwTx, shardId types.ShardId, hash common.Hash, code types.Code) error {
	return tx.PutToShard(shardId, codeTable, hash.Bytes(), code[:])
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
