package db

import (
	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	"log"
)

func ReadBlockSSZ(tx Tx, hash common.Hash, number uint64) []byte {
	data, err := tx.GetOne(BlockTable, hash.Bytes())
	if err != nil {
		log.Fatal("ReadHeaderRLP failed", "err", err)
	}
	return data
}

func ReadBlock(tx Tx, hash common.Hash, number uint64) *types.Block {
	data := ReadBlockSSZ(tx, hash, number)
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

	// Write the encoded header
	data, err := block.EncodeSSZ(nil)

	if err != nil {
		return err
	}
	if err := tx.Put(BlockTable, hash.Bytes(), data); err != nil {
		return err
	}
	return nil
}
