package jsonrpc

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func writeTestBlock(t *testing.T, tx db.RwTx, shardId types.ShardId, blockNumber types.BlockNumber,
	messages []*types.Message, receipts []*types.Receipt, outMessages []*types.Message,
) common.Hash {
	t.Helper()
	block := types.Block{
		Id:                  blockNumber,
		PrevBlock:           common.EmptyHash,
		SmartContractsRoot:  common.EmptyHash,
		InMessagesRoot:      writeMessages(t, tx, shardId, messages).RootHash(),
		OutMessagesRoot:     writeMessages(t, tx, shardId, outMessages).RootHash(),
		ReceiptsRoot:        writeReceipts(t, tx, shardId, receipts).RootHash(),
		OutMessagesNum:      types.MessageIndex(len(outMessages)),
		ChildBlocksRootHash: common.EmptyHash,
		MainChainHash:       common.EmptyHash,
	}
	hash := block.Hash()
	require.NoError(t, db.WriteBlock(tx, types.BaseShardId, hash, &block))
	return hash
}

func writeMessages(t *testing.T, tx db.RwTx, shardId types.ShardId, messages []*types.Message) *execution.MessageTrie {
	t.Helper()
	messageRoot := execution.NewDbMessageTrie(tx, shardId)
	for i, message := range messages {
		require.NoError(t, messageRoot.Update(types.MessageIndex(i), message))
	}
	return messageRoot
}

func writeReceipts(t *testing.T, tx db.RwTx, shardId types.ShardId, receipts []*types.Receipt) *execution.ReceiptTrie {
	t.Helper()
	receiptRoot := execution.NewDbReceiptTrie(tx, shardId)
	for i, receipt := range receipts {
		require.NoError(t, receiptRoot.Update(types.MessageIndex(i), receipt))
	}
	return receiptRoot
}
