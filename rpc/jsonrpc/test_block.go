package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

func writeTestBlock(t *testing.T, tx db.RwTx, shardId types.ShardId, blockNumber types.BlockNumber, messages []*types.Message, receipts []*types.Receipt) common.Hash {
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
	require.NoError(t, db.WriteBlock(tx, types.BaseShardId, &block))
	return block.Hash()
}

func writeMessages(t *testing.T, tx db.RwTx, shardId types.ShardId, messages []*types.Message) *execution.MessageTrie {
	t.Helper()
	messageRoot := execution.NewMessageTrie(mpt.NewMerklePatriciaTrie(tx, shardId, db.MessageTrieTable))
	for i, message := range messages {
		require.NoError(t, messageRoot.Update(types.MessageIndex(i), message))
	}
	return messageRoot
}

func writeReceipts(t *testing.T, tx db.RwTx, shardId types.ShardId, receipts []*types.Receipt) *execution.ReceiptTrie {
	t.Helper()
	receiptRoot := execution.NewReceiptTrie(mpt.NewMerklePatriciaTrie(tx, shardId, db.ReceiptTrieTable))
	for i, receipt := range receipts {
		require.NoError(t, receiptRoot.Update(types.MessageIndex(i), receipt))
	}
	return receiptRoot
}
