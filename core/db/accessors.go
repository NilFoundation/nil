package db

import (
	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var logger = common.NewLogger("DB", false /* noColor */)

func readBlockRaw(tx Tx, hash common.Hash) *[]byte {
	data, err := tx.Get(BlockTable, hash.Bytes())
	if err != nil {
		logger.Fatal().Msgf("ReadHeaderRLP failed. err: %s", err.Error())
	}
	return data
}

func ReadBlock(tx Tx, hash common.Hash) *types.Block {
	data := readBlockRaw(tx, hash)
	if data == nil {
		return nil
	}
	header := new(types.Block)
	err := header.DecodeSSZ(*data, 0)

	if err != nil {
		logger.Fatal().Msgf("Invalid block header RLP. hash: %v, err: %v", hash, err)
	}
	return header
}

func WriteBlock(tx Tx, block *types.Block) error {
	hash := block.Hash()

	encoded := new([]byte)
	if err := block.EncodeSSZ(encoded); err != nil {
		return err
	}

	if err := tx.Put(BlockTable, hash.Bytes(), *encoded); err != nil {
		return err
	}
	return nil
}

func readContractRaw(tx Tx, hash common.Hash) *[]byte {
	data, err := tx.Get(ContractTable, hash.Bytes())
	if err != nil {
		logger.Fatal().Msgf("readAccountRaw failed. err: %s", err.Error())
	}
	return data
}

func ReadContract(tx Tx, hash common.Hash) *types.SmartContract {
	data := readContractRaw(tx, hash)
	if data == nil {
		return nil
	}
	contract := new(types.SmartContract)
	err := contract.DecodeSSZ(*data, 0)

	if err != nil {
		logger.Fatal().Msgf("Invalid contract encoding. hash: %v, err: %v", hash, err)
	}
	return contract
}

func WriteContract(tx Tx, contract *types.SmartContract) error {
	hash := contract.Hash()

	data, err := contract.EncodeSSZ(nil)

	if err != nil {
		return err
	}
	if err := tx.Put(ContractTable, hash.Bytes(), data); err != nil {
		return err
	}
	return nil
}

func WriteCode(tx Tx, code types.Code) error {
	hash := code.Hash()
	if err := tx.Put(CodeTable, hash.Bytes(), code[:]); err != nil {
		return err
	}
	return nil
}

func ReadCode(tx Tx, hash common.Hash) (*types.Code, error) {
	code, err := tx.Get(StorageTable, hash[:])
	if err != nil {
		return nil, err
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
