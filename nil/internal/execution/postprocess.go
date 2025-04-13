package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func PostprocessBlock(tx db.RwTx, shardId types.ShardId, blockResult *BlockGenerationResult, mode string) error {
	if blockResult.Block == nil {
		return errors.New("block is not set")
	}
	postprocessor := blockPostprocessor{tx, shardId, blockResult, mode}
	return postprocessor.Postprocess()
}

type blockPostprocessor struct {
	tx          db.RwTx
	shardId     types.ShardId
	blockResult *BlockGenerationResult
	execMode    string
}

func (pp *blockPostprocessor) Postprocess() error {
	for _, postpocessor := range []func() error{
		pp.fillLastBlockTable,
		pp.fillBlockHashByNumberIndex,
		pp.fillBlockHashAndTransactionIndexByTransactionHash,
	} {
		if err := postpocessor(); err != nil {
			return err
		}
	}
	return nil
}

func (pp *blockPostprocessor) fillLastBlockTable() error {
	return db.WriteLastBlockHash(pp.tx, pp.shardId, pp.blockResult.BlockHash)
}

func (pp *blockPostprocessor) fillBlockHashByNumberIndex() error {
	return pp.tx.PutToShard(
		pp.shardId, db.BlockHashByNumberIndex, pp.blockResult.Block.Id.Bytes(), pp.blockResult.BlockHash.Bytes())
}

func (pp *blockPostprocessor) fillBlockHashAndTransactionIndexByTransactionHash() error {
	fill := func(txnHashes []common.Hash, table db.ShardedTableName) error {
		for i, hash := range txnHashes {
			blockHashAndTransactionIndex := db.BlockHashAndTransactionIndex{
				BlockHash:        pp.blockResult.BlockHash,
				TransactionIndex: types.TransactionIndex(i),
			}
			value, err := blockHashAndTransactionIndex.MarshalSSZ()
			if err != nil {
				return err
			}

			// In manual replay mode all transactions should be already in DB
			if assert.Enable && pp.execMode != ModeManualReplay {
				ok, err := pp.tx.ExistsInShard(pp.shardId, table, hash.Bytes())
				if err != nil {
					return fmt.Errorf("fail to check transaction existence in DB: %w", err)
				}
				if ok {
					return fmt.Errorf("fatal: duplicate transaction in %s: %x", table, hash)
				}
			}

			if err := pp.tx.PutToShard(pp.shardId, table, hash.Bytes(), value); err != nil {
				return err
			}
		}
		return nil
	}

	if err := fill(pp.blockResult.InTxnHashes, db.BlockHashAndInTransactionIndexByTransactionHash); err != nil {
		return err
	}
	return fill(pp.blockResult.OutTxnHashes, db.BlockHashAndOutTransactionIndexByTransactionHash)
}
