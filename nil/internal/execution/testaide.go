//go:build test

package execution

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

var (
	MainPrivateKey   *ecdsa.PrivateKey
	MainPublicKey    []byte
	DefaultGasCredit = types.Gas(DefaultGasLimit).ToValue(types.DefaultGasPrice)
	DefaultSendValue = types.GasToValue(200_000_000)
)

const (
	DefaultGasLimit = 100_000
)

func init() {
	var err error
	MainPrivateKey, MainPublicKey, err = nilcrypto.GenerateKeyPair()
	check.PanicIfErr(err)
}

func GenerateZeroState(t *testing.T, shardId types.ShardId, txFabric db.DB) *types.Block {
	t.Helper()

	g, err := NewBlockGenerator(t.Context(),
		NewBlockGeneratorParams(shardId, 1),
		txFabric, nil)
	require.NoError(t, err)
	defer g.Rollback()

	zerostateCfg, err := CreateDefaultZeroStateConfig(MainPublicKey)
	require.NoError(t, err)
	zerostateCfg.ConfigParams = ConfigParams{
		GasPrice: config.ParamGasPrice{
			Shards: []types.Uint256{*types.NewUint256(10), *types.NewUint256(10), *types.NewUint256(10)},
		},
	}
	block, err := g.GenerateZeroState(zerostateCfg)
	require.NoError(t, err)
	require.NotNil(t, block)
	return block
}

func GenerateBlockFromTransactions(t *testing.T,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, childShardBlocks map[types.ShardId]common.Hash, txns ...*types.Transaction,
) common.Hash {
	t.Helper()
	return generateBlockFromTransactions(t, true, shardId, blockId, prevBlock, txFabric, childShardBlocks, txns...)
}

func GenerateBlockFromTransactionsWithoutExecution(t *testing.T,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, txns ...*types.Transaction,
) common.Hash {
	t.Helper()
	return generateBlockFromTransactions(t, false, shardId, blockId, prevBlock, txFabric, nil, txns...)
}

func generateBlockFromTransactions(t *testing.T, execute bool,
	shardId types.ShardId, blockId types.BlockNumber, prevBlockHash common.Hash,
	txFabric db.DB, childShardBlocks map[types.ShardId]common.Hash, txns ...*types.Transaction,
) common.Hash {
	t.Helper()

	tx, err := txFabric.CreateRwTx(t.Context())
	require.NoError(t, err)
	defer tx.Rollback()

	prevBlock, err := db.ReadBlock(tx, shardId, prevBlockHash)
	if err != nil {
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	} else {
		require.NoError(t, err)
	}

	es, err := NewExecutionState(tx, shardId, StateParams{
		Block:          prevBlock,
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)
	es.BaseFee = types.DefaultGasPrice

	for _, txn := range txns {
		txn.TxId = es.InTxCounts[txn.From.ShardId()]
		es.AddInTransaction(txn)

		if !execute {
			es.AddReceipt(NewExecutionResult())
			continue
		}

		var execResult *ExecutionResult
		if txn.IsRefund() {
			execResult = NewExecutionResult()
			execResult.SetFatal(es.handleRefundTransaction(t.Context(), txn))
		} else {
			execResult = es.HandleTransaction(t.Context(), txn, dummyPayer{})
		}
		require.False(t, execResult.Failed())

		es.AddReceipt(execResult)
	}

	es.ChildShardBlocks = childShardBlocks

	blockRes, err := es.Commit(blockId, nil)
	require.NoError(t, err)

	err = PostprocessBlock(tx, shardId, blockRes, ModeVerify)
	require.NoError(t, err)
	require.NotNil(t, blockRes.Block)

	require.NoError(t, db.WriteBlockTimestamp(tx, shardId, blockRes.BlockHash, 0))

	require.NoError(t, tx.Commit())

	return blockRes.BlockHash
}

func NewDeployTransaction(payload types.DeployPayload,
	shardId types.ShardId, from types.Address, seqno types.Seqno, value types.Value,
) *types.Transaction {
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:        types.NewTransactionFlags(types.TransactionFlagInternal, types.TransactionFlagDeploy),
			Data:         payload.Bytes(),
			Seqno:        seqno,
			FeeCredit:    types.GasToValue(10_000_000),
			To:           types.CreateAddress(shardId, payload),
			MaxFeePerGas: types.MaxFeePerGasDefault,
		},
		From:  from,
		Value: value,
	}
}

func NewExecutionTransaction(from, to types.Address, seqno types.Seqno, callData []byte) *types.Transaction {
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			To:           to,
			Data:         callData,
			Seqno:        seqno,
			FeeCredit:    DefaultGasCredit,
			MaxFeePerGas: types.MaxFeePerGasDefault,
		},
		From: from,
	}
}

func NewSendMoneyTransaction(t *testing.T, to types.Address, seqno types.Seqno) *types.Transaction {
	t.Helper()

	m := NewExecutionTransaction(types.MainSmartAccountAddress, types.MainSmartAccountAddress, seqno,
		contracts.NewSmartAccountSendCallData(t, types.Code{},
			DefaultGasLimit, DefaultSendValue,
			[]types.TokenBalance{}, to, types.ExecutionTransactionKind))
	require.NoError(t, m.Sign(MainPrivateKey))

	return m
}

func Deploy(t *testing.T, es *ExecutionState,
	payload types.DeployPayload, shardId types.ShardId, from types.Address, seqno types.Seqno,
) types.Address {
	t.Helper()

	txn := NewDeployTransaction(payload, shardId, from, seqno, types.Value{})
	execResult := es.AddAndHandleTransaction(t.Context(), txn, dummyPayer{})
	require.False(t, execResult.Failed())
	es.AddReceipt(execResult)

	return txn.To
}

func (es *ExecutionState) AddAndHandleTransaction(
	ctx context.Context, txn *types.Transaction, payer Payer,
) *ExecutionResult {
	txn.TxId = es.InTxCounts[txn.From.ShardId()]
	es.AddInTransaction(txn)
	return es.HandleTransaction(ctx, txn, payer)
}
