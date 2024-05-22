package execution

import (
	"fmt"
	"strconv"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	ssz "github.com/ferranbt/fastssz"
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
		pp.fillMessageHashByBlockIdAndMessageIdIndex,
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

func (pp *blockPostprocessor) fillMessageHashByBlockIdAndMessageIdIndex() error {
	mptMessages := mpt.NewMerklePatriciaTrieWithRoot(pp.tx, pp.shardId, db.MessageTrieTable, pp.block.MessagesRoot)
	for kv := range mptMessages.Iterate() {
		messageIndex := ssz.UnmarshallUint64(kv.Key)

		var message types.Message
		if err := message.UnmarshalSSZ(kv.Value); err != nil {
			return err
		}
		messageHash := message.Hash()

		key := BlockIdAndMessageId{pp.block.Id, messageIndex}.AsTableKey()
		if err := pp.tx.PutToShard(pp.shardId, db.MessageHashByBlockIdAndMessageIdIndex, key, messageHash.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

type BlockIdAndMessageId struct {
	blockId   types.BlockNumber
	messageId uint64 // TODO: make MessageId strong typed
}

func (bm BlockIdAndMessageId) AsTableKey() []byte {
	return strconv.AppendUint(append(bm.blockId.Bytes(), ':'), bm.messageId, 10)
}
