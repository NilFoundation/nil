//go:build test

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
	transactions []*types.Transaction, receipts []*types.Receipt, outTransactions []*types.Transaction,
) *execution.BlockGenerationResult {
	t.Helper()
	block := types.Block{
		BlockData: types.BlockData{
			Id:                  blockNumber,
			PrevBlock:           common.EmptyHash,
			SmartContractsRoot:  common.EmptyHash,
			InTransactionsRoot:  writeTransactions(t, tx, shardId, transactions),
			OutTransactionsRoot: writeTransactions(t, tx, shardId, outTransactions),
			ReceiptsRoot:        writeReceipts(t, tx, shardId, receipts),
			OutTransactionsNum:  types.TransactionIndex(len(outTransactions)),
			ChildBlocksRootHash: common.EmptyHash,
			MainShardHash:       common.EmptyHash,
		},
	}
	hash := block.Hash(types.BaseShardId)
	require.NoError(t, db.WriteBlock(tx, types.BaseShardId, hash, &block))

	require.Len(t, receipts, len(transactions))
	inTxnHashes := make([]common.Hash, len(transactions))
	for i, r := range receipts {
		inTxnHashes[i] = r.TxnHash
	}
	outTxnHashes := make([]common.Hash, len(outTransactions))
	for i, txn := range outTransactions {
		outTxnHashes[i] = txn.Hash()
	}
	return &execution.BlockGenerationResult{
		BlockHash:    hash,
		Block:        &block,
		InTxns:       transactions,
		InTxnHashes:  inTxnHashes,
		OutTxns:      outTransactions,
		OutTxnHashes: outTxnHashes,
	}
}

func writeTransactions(
	t *testing.T,
	tx db.RwTx,
	shardId types.ShardId,
	transactions []*types.Transaction,
) common.Hash {
	t.Helper()
	transactionRoot := execution.NewDbTransactionTrie(tx, shardId)
	for i, transaction := range transactions {
		require.NoError(t, transactionRoot.Update(types.TransactionIndex(i), transaction))
	}
	root, err := transactionRoot.Commit()
	require.NoError(t, err)
	return root
}

func writeReceipts(t *testing.T, tx db.RwTx, shardId types.ShardId, receipts []*types.Receipt) common.Hash {
	t.Helper()
	receiptRoot := execution.NewDbReceiptTrie(tx, shardId)
	for i, receipt := range receipts {
		require.NoError(t, receiptRoot.Update(types.TransactionIndex(i), receipt))
	}
	root, err := receiptRoot.Commit()
	require.NoError(t, err)
	return root
}
