package db

import (
	"log"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

func ReadBlockSSZ(tx Tx, hash common.Hash) []byte {
	data, err := tx.GetOne(BlockTable, hash.Bytes())
	if err != nil {
		log.Fatal("ReadHeaderRLP failed", "err", err)
	}
	return data
}

func ReadBlock(tx Tx, hash common.Hash) *types.Block {
	data := ReadBlockSSZ(tx, hash)
	if len(data) == 0 {
		return nil
	}
	header := new(types.Block)
	err := header.DecodeSSZ(data, 0)

	if err != nil {
		log.Fatal("Invalid block header RLP", "hash", hash, "err", err)
	}
	return header
}

func WriteBlock(tx Tx, block *types.Block) error {
	hash := block.Hash()

	data, err := block.EncodeSSZ(nil)

	if err != nil {
		return err
	}
	if err := tx.Put(BlockTable, hash.Bytes(), data); err != nil {
		return err
	}
	return nil
}

func WriteStorage(tx Tx, addr common.Address, key common.Hash, value uint256.Int) error {
	fullKey := make([]byte, common.AddrSize+common.HashSize)
	copy(fullKey, addr[:])
	copy(fullKey[common.HashSize:], key[:])

	v := value.Bytes()
	if len(v) == 0 {
		return tx.Delete(db.StorageTrieTable, fullKey)
	}

	return tx.Put(db.StorageTrieTable, fullKey, v)
}
