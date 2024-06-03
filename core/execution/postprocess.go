package execution

import (
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
)

func PostprocessBlock(tx db.RwTx, shardId types.ShardId, blockHash common.Hash) (*types.Block, error) {
	postprocessor, err := newBlockPostprocessor(tx, shardId, blockHash)
	if err != nil {
		return nil, err
	}
	return postprocessor.block, postprocessor.Postprocess()
}

type blockPostprocessor struct {
	tx        db.RwTx
	shardId   types.ShardId
	blockHash common.Hash
	block     *types.Block
}

func newBlockPostprocessor(tx db.RwTx, shardId types.ShardId, blockHash common.Hash) (blockPostprocessor, error) {
	block := db.ReadBlock(tx, shardId, blockHash)
	if block == nil {
		return blockPostprocessor{}, fmt.Errorf("Block %s not found", blockHash.String())
	}
	return blockPostprocessor{tx, shardId, blockHash, block}, nil
}

func (pp *blockPostprocessor) Postprocess() error {
	for _, postpocessor := range []func() error{
		pp.fillLastBlockTable,
		pp.fillBlockHashByNumberIndex,
		pp.fillBlockHashAndMessageIndexByMessageHash,
	} {
		if err := postpocessor(); err != nil {
			return err
		}
	}
	return nil
}

func (pp *blockPostprocessor) fillLastBlockTable() error {
	if err := pp.tx.Put(db.LastBlockTable, pp.shardId.Bytes(), pp.blockHash[:]); err != nil {
		return err
	}
	return nil
}

func (pp *blockPostprocessor) fillBlockHashByNumberIndex() error {
	if err := pp.tx.PutToShard(pp.shardId, db.BlockHashByNumberIndex, pp.block.Id.Bytes(), pp.blockHash.Bytes()); err != nil {
		return err
	}
	return nil
}

func (pp *blockPostprocessor) fillBlockHashAndMessageIndexByMessageHash() error {
	// TODO: fix for out messages
	mptMessages := mpt.NewMerklePatriciaTrieWithRoot(pp.tx, pp.shardId, db.MessageTrieTable, pp.block.InMessagesRoot)

	// TODO: currently "Iterate" works via channel.
	// It probably causes concurrent usage of "tx" object that
	// triggers race detector. So split logic in two steps that should be safer.
	messages := make([]mpt.MptIteratorKey, 0)
	for kv := range mptMessages.Iterate() {
		messages = append(messages, kv)
	}

	for _, kv := range messages {
		messageIndex := types.BytesToMessageIndex(kv.Key)

		var message types.Message
		if err := message.UnmarshalSSZ(kv.Value); err != nil {
			return err
		}
		messageHash := message.Hash()

		blockHashAndMessageIndex := db.BlockHashAndMessageIndex{BlockHash: pp.blockHash, MessageIndex: messageIndex}
		value, err := blockHashAndMessageIndex.MarshalSSZ()
		if err != nil {
			return err
		}

		if err := pp.tx.PutToShard(pp.shardId, db.BlockHashAndMessageIndexByMessageHash, messageHash.Bytes(), value); err != nil {
			return err
		}
	}
	return nil
}
