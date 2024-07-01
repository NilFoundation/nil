package execution

import (
	"errors"
	"math"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
)

func PostprocessBlock(tx db.RwTx, shardId types.ShardId, defaultGasPrice types.Value, gasPriceScale float64, blockHash common.Hash) (*types.Block, error) {
	postprocessor, err := newBlockPostprocessor(tx, shardId, defaultGasPrice, gasPriceScale, blockHash)
	if err != nil {
		return nil, err
	}
	return postprocessor.block, postprocessor.Postprocess()
}

type blockPostprocessor struct {
	tx              db.RwTx
	shardId         types.ShardId
	blockHash       common.Hash
	block           *types.Block
	defaultGasPrice types.Value
	gasPriceScale   float64
}

func newBlockPostprocessor(tx db.RwTx, shardId types.ShardId, defaultGasPrice types.Value, gasPriceScale float64, blockHash common.Hash) (*blockPostprocessor, error) {
	block, err := db.ReadBlock(tx, shardId, blockHash)
	if err != nil {
		return nil, err
	}
	return &blockPostprocessor{tx, shardId, blockHash, block, defaultGasPrice, gasPriceScale}, nil
}

func (pp *blockPostprocessor) Postprocess() error {
	for _, postpocessor := range []func() error{
		pp.fillLastBlockTable,
		pp.fillBlockHashByNumberIndex,
		pp.fillBlockHashAndMessageIndexByMessageHash,
		pp.updateGasPrice,
	} {
		if err := postpocessor(); err != nil {
			return err
		}
	}
	return nil
}

func (pp *blockPostprocessor) updateGasPrice() error {
	decreasePerBlock := types.NewValueFromUint64(1)
	maxGasPrice := types.NewValueFromUint64(100)

	gasPrice, err := db.ReadGasPerShard(pp.tx, pp.shardId)
	if errors.Is(err, db.ErrKeyNotFound) {
		gasPrice = pp.defaultGasPrice
	} else if err != nil {
		return err
	}

	gasIncrease := uint64(math.Ceil(float64(pp.block.OutMessagesNum) * pp.gasPriceScale))
	newGasPrice := gasPrice.Add(types.NewValueFromUint64(gasIncrease))
	// Check if new gas price is less than the current one (overflow case) or greater than the max allowed
	if gasPrice.Cmp(newGasPrice) > 0 || newGasPrice.Cmp(maxGasPrice) > 0 {
		gasPrice = maxGasPrice
	} else {
		gasPrice = newGasPrice
	}
	if gasPrice.Cmp(decreasePerBlock) >= 0 {
		gasPrice = gasPrice.Sub(decreasePerBlock)
	}
	if gasPrice.Cmp(pp.defaultGasPrice) < 0 {
		gasPrice = pp.defaultGasPrice
	}
	return db.WriteGasPerShard(pp.tx, pp.shardId, gasPrice)
}

func (pp *blockPostprocessor) fillLastBlockTable() error {
	return db.WriteLastBlockHash(pp.tx, pp.shardId, pp.blockHash)
}

func (pp *blockPostprocessor) fillBlockHashByNumberIndex() error {
	if err := pp.tx.PutToShard(pp.shardId, db.BlockHashByNumberIndex, pp.block.Id.Bytes(), pp.blockHash.Bytes()); err != nil {
		return err
	}
	return nil
}

func (pp *blockPostprocessor) fillBlockHashAndMessageIndexByMessageHash() error {
	fill := func(root common.Hash, table db.ShardedTableName) error {
		mptMessages := NewMessageTrieReader(mpt.NewReaderWithRoot(pp.tx, pp.shardId, db.MessageTrieTable, root))
		msgs, err := mptMessages.Entries()
		if err != nil {
			return err
		}

		for _, kv := range msgs {
			blockHashAndMessageIndex := db.BlockHashAndMessageIndex{BlockHash: pp.blockHash, MessageIndex: kv.Key}
			value, err := blockHashAndMessageIndex.MarshalSSZ()
			if err != nil {
				return err
			}

			if err := pp.tx.PutToShard(pp.shardId, table, kv.Val.Hash().Bytes(), value); err != nil {
				return err
			}
		}
		return nil
	}

	if err := fill(pp.block.InMessagesRoot, db.BlockHashAndInMessageIndexByMessageHash); err != nil {
		return err
	}
	return fill(pp.block.OutMessagesRoot, db.BlockHashAndOutMessageIndexByMessageHash)
}
