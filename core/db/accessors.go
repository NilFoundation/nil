package db

import (
	"errors"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

func readDecodable[
	S any,
	T interface {
		~*S
		ssz.SSZDecodable
	},
](tx Tx, table string, hash common.Hash) *S {
	data, err := tx.Get(table, hash.Bytes())
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
](tx Tx, table string, obj T) error {
	hash := obj.Hash()

	data, err := obj.EncodeSSZ(nil)
	if err != nil {
		return err
	}

	return tx.Put(table, hash.Bytes(), data)
}

func ReadBlock(tx Tx, hash common.Hash) *types.Block {
	return readDecodable[types.Block, *types.Block](tx, BlockTable, hash)
}

func WriteBlock(tx Tx, block *types.Block) error {
	return writeEncodable(tx, BlockTable, block)
}

func ReadContract(tx Tx, hash common.Hash) *types.SmartContract {
	return readDecodable[types.SmartContract, *types.SmartContract](tx, ContractTable, hash)
}

func WriteContract(tx Tx, contract *types.SmartContract) error {
	return writeEncodable(tx, ContractTable, contract)
}

func WriteCode(tx Tx, code types.Code) error {
	hash := code.Hash()
	if err := tx.Put(CodeTable, hash.Bytes(), code[:]); err != nil {
		return err
	}
	return nil
}

func ReadCode(tx Tx, hash common.Hash) (*types.Code, error) {
	code, err := tx.Get(CodeTable, hash[:])
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, nil
	}

	res := types.Code(*code)
	return &res, nil
}

func WriteStorage(tx Tx, addr common.Address, key common.Hash, value uint256.Int) error {
	fullKey := make([]byte, common.AddrSize+common.HashSize)
	copy(fullKey, addr[:])
	copy(fullKey[common.HashSize:], key[:])

	v := value.Bytes()
	if len(v) == 0 {
		return tx.Delete(StorageTable, fullKey)
	}

	return tx.Put(StorageTable, fullKey, v)
}

func ReadStorage(tx Tx, addr common.Address, key common.Hash) (*uint256.Int, error) {
	fullKey := make([]byte, common.AddrSize+common.HashSize)
	copy(fullKey, addr[:])
	copy(fullKey[common.HashSize:], key[:])

	enc, err := tx.Get(StorageTable, fullKey)

	if enc == nil {
		return nil, err
	}

	var res uint256.Int
	res.SetBytes(*enc)

	return &res, nil
}
