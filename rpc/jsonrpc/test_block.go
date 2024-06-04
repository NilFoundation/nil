package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	ssz "github.com/ferranbt/fastssz"
	"github.com/stretchr/testify/require"
)

func writeTestBlock(t *testing.T, tx db.Tx, shardId types.ShardId, blockNumber types.BlockNumber, messages []*types.Message, receipts []*types.Receipt) common.Hash {
	t.Helper()
	block := types.Block{
		Id:                  blockNumber,
		PrevBlock:           common.EmptyHash,
		SmartContractsRoot:  common.EmptyHash,
		InMessagesRoot:      writeMessages(t, tx, shardId, messages).RootHash(),
		ReceiptsRoot:        writeReceipts(t, tx, shardId, receipts).RootHash(),
		ChildBlocksRootHash: common.EmptyHash,
		MasterChainHash:     common.EmptyHash,
	}
	require.NoError(t, db.WriteBlock(tx, types.MasterShardId, &block))
	return block.Hash()
}

func writeMessages(t *testing.T, tx db.Tx, shardId types.ShardId, messages []*types.Message) *mpt.MerklePatriciaTrie {
	t.Helper()
	messageRoot := mpt.NewMerklePatriciaTrie(tx, shardId, db.MessageTrieTable)
	for i, message := range messages {
		messageBytes, err := message.MarshalSSZ()
		require.NoError(t, err)
		require.NoError(t, messageRoot.Set(ssz.MarshalUint64(nil, uint64(i)), messageBytes))
	}
	return messageRoot
}

func writeReceipts(t *testing.T, tx db.Tx, shardId types.ShardId, receipts []*types.Receipt) *mpt.MerklePatriciaTrie {
	t.Helper()
	receiptRoot := mpt.NewMerklePatriciaTrie(tx, shardId, db.ReceiptTrieTable)
	for i, receipt := range receipts {
		receiptBytes, err := receipt.MarshalSSZ()
		require.NoError(t, err)
		require.NoError(t, receiptRoot.Set(ssz.MarshalUint64(nil, uint64(i)), receiptBytes))
	}
	return receiptRoot
}
