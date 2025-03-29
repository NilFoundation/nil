package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestDebugGetBlock(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	txn := types.NewEmptyTransaction()
	txnHash := txn.Hash()
	errStr := "test error"
	inTransactionTree := execution.NewDbTransactionTrie(tx, types.MainShardId)
	require.NoError(t, inTransactionTree.Update(types.TransactionIndex(0), txn))
	require.NoError(t, db.WriteError(tx, txnHash, errStr))

	blockWithErrors := &types.Block{
		BlockData: types.BlockData{
			Id:                 258,
			InTransactionsRoot: inTransactionTree.RootHash(),
		},
	}
	b1 := &execution.BlockGenerationResult{
		Block:       blockWithErrors,
		BlockHash:   blockWithErrors.Hash(types.MainShardId),
		InTxns:      []*types.Transaction{txn},
		InTxnHashes: []common.Hash{txnHash},
	}

	block := &types.Block{
		BlockData: types.BlockData{
			Id: 259,
		},
	}
	b2 := &execution.BlockGenerationResult{
		Block:     block,
		BlockHash: block.Hash(types.MainShardId),
	}

	var hexBytes []byte
	for _, b := range []*execution.BlockGenerationResult{b1, b2} {
		hexBytes, err = b.Block.MarshalSSZ()
		require.NoError(t, err)

		err = db.WriteBlock(tx, types.MainShardId, b.BlockHash, b.Block)
		require.NoError(t, err)

		err = execution.PostprocessBlock(tx, types.MainShardId, b, execution.ModeVerify)
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)

	mainShardApi := rawapi.NewLocalShardApi(types.MainShardId, database, nil, false)
	localShardApis := map[types.ShardId]rawapi.ShardApi{
		types.MainShardId: mainShardApi,
	}
	localApi := rawapi.NewNodeApiOverShardApis(localShardApis)
	api := NewDebugAPI(localApi, logging.GlobalLogger)

	// When: Get the latest block
	res1, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)

	content := res1.Content
	require.EqualValues(t, hexBytes, content)

	// When: Get existing block
	res2, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id), false)
	require.NoError(t, err)

	require.Equal(t, res1, res2)

	// When: Get nonexistent block
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id+1), false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	// When: Get existing block with additional data
	res3, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(blockWithErrors.Id), true)
	require.NoError(t, err)
	require.Len(t, res3.InTransactions, 1)
	require.Len(t, res3.Errors, 1)
	require.Equal(t, errStr, res3.Errors[txnHash])

	// When: Get existing block without additional data
	res4, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(blockWithErrors.Id), false)
	require.NoError(t, err)
	require.Empty(t, res4.InTransactions)
}

type SuiteDbgContracts struct {
	SuiteAccountsBase
	debugApi *DebugAPIImpl
}

func (suite *SuiteDbgContracts) SetupSuite() {
	suite.SuiteAccountsBase.SetupSuite()

	shardId := types.BaseShardId
	ctx := context.Background()

	var err error
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	suite.smcAddr = types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))
	suite.Require().NoError(es.SetCode(suite.smcAddr, []byte("some code")))
	suite.Require().NoError(es.SetState(suite.smcAddr, common.Hash{0x1}, common.IntToHash(2)))
	suite.Require().NoError(es.SetState(suite.smcAddr, common.Hash{0x3}, common.IntToHash(4)))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetExtSeqno(suite.smcAddr, 567))

	blockRes, err := es.Commit(0, nil)
	suite.Require().NoError(err)
	suite.blockHash = blockRes.BlockHash

	err = execution.PostprocessBlock(tx, shardId, blockRes, execution.ModeVerify)
	suite.Require().NotNil(blockRes.Block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	shardApi := rawapi.NewLocalShardApi(shardId, suite.db, nil, false)
	localShardApis := map[types.ShardId]rawapi.ShardApi{
		shardId: shardApi,
	}
	localApi := rawapi.NewNodeApiOverShardApis(localShardApis)
	suite.debugApi = NewDebugAPI(localApi, logging.NewLogger("Test"))
	suite.Require().NoError(err)
}

func (suite *SuiteDbgContracts) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteDbgContracts) TestGetContract() {
	ctx := context.Background()
	res, err := suite.debugApi.GetContract(
		ctx,
		suite.smcAddr,
		transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber})
	suite.Require().NoError(err)

	suite.Run("storage", func() {
		expected := map[common.Hash]types.Uint256{
			{0x1}: *types.NewUint256(2),
			{0x3}: *types.NewUint256(4),
		}
		suite.Require().Equal(expected, res.Storage)
	})

	suite.Run("proof", func() {
		tx, err := suite.db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		shardId := suite.smcAddr.ShardId()
		accessor := execution.NewStateAccessor().Access(tx, shardId).GetBlock()
		data, err := accessor.ByHash(suite.blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(data.Block())

		contractRawReader := mpt.NewDbReader(tx, shardId, db.ContractTrieTable)
		contractRawReader.SetRootHash(data.Block().SmartContractsRoot)

		expectedContract, err := contractRawReader.Get(suite.smcAddr.Hash().Bytes())
		suite.Require().NoError(err)

		proof, err := mpt.DecodeProof(res.Proof)
		suite.Require().NoError(err)

		ok, err := proof.VerifyRead(suite.smcAddr.Hash().Bytes(), expectedContract, data.Block().SmartContractsRoot)
		suite.Require().NoError(err)
		suite.Require().True(ok)
	})
}

func TestSuiteDbgContracts(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteDbgContracts))
}
