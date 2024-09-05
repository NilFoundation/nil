package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestDebugGetBlock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	msg := &types.Message{Seqno: 0}
	errStr := "test error"
	inMessageTree := execution.NewDbMessageTrie(tx, types.MainShardId)
	require.NoError(t, inMessageTree.Update(types.MessageIndex(0), msg))
	require.NoError(t, db.WriteError(tx, msg.Hash(), errStr))

	blockWithErrors := &types.Block{
		Id:                 258,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
		InMessagesRoot:     inMessageTree.RootHash(),
	}

	block := &types.Block{
		Id:                 259,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
	}

	var blockHex string
	for _, b := range []*types.Block{blockWithErrors, block} {
		hexBytes, err := b.MarshalSSZ()
		require.NoError(t, err)
		blockHex = hexutil.Encode(hexBytes)

		err = db.WriteBlock(tx, types.MainShardId, b)
		require.NoError(t, err)

		_, err = execution.PostprocessBlock(tx, types.MainShardId, types.NewValueFromUint64(10), 0, b.Hash())
		require.NoError(t, err)
	}

	err = tx.Commit()
	require.NoError(t, err)

	base := NewBaseApi(0)
	api := NewDebugAPI(base, database, log.Logger)

	// When: Get the latest block
	res1, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)

	content := res1.Content
	require.Equal(t, blockHex, content)

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
	require.Len(t, res3.Errors, 1)
	require.Equal(t, errStr, res3.Errors[msg.Hash()])
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

	es, err := execution.NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0), 1)
	suite.Require().NoError(err)

	suite.smcAddr = types.GenerateRandomAddress(shardId)
	suite.Require().NotEmpty(suite.smcAddr)

	suite.Require().NoError(es.CreateAccount(suite.smcAddr))
	suite.Require().NoError(es.SetCode(suite.smcAddr, []byte("some code")))
	suite.Require().NoError(es.SetState(suite.smcAddr, common.Hash{0x1}, common.IntToHash(2)))
	suite.Require().NoError(es.SetState(suite.smcAddr, common.Hash{0x3}, common.IntToHash(4)))

	suite.Require().NoError(es.SetBalance(suite.smcAddr, types.NewValueFromUint64(1234)))
	suite.Require().NoError(es.SetExtSeqno(suite.smcAddr, 567))

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	block, err := execution.PostprocessBlock(tx, shardId, types.NewValueFromUint64(10), 0, blockHash)
	suite.Require().NotNil(block)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.debugApi = NewDebugAPI(
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, logging.NewLogger("Test"))
	suite.Require().NoError(err)
}

func (suite *SuiteDbgContracts) TearDownSuite() {
	suite.SuiteAccountsBase.TearDownSuite()
}

func (suite *SuiteDbgContracts) TestGetContract() {
	ctx := context.Background()
	res, err := suite.debugApi.GetContract(ctx, suite.smcAddr, transport.BlockNumberOrHash{BlockNumber: transport.LatestBlock.BlockNumber})
	suite.Require().NoError(err)

	suite.Run("storage", func() {
		expected := []execution.Entry[common.Hash, *types.Uint256]{
			{
				Key: common.Hash{0x1},
				Val: types.NewUint256(2),
			},
			{
				Key: common.Hash{0x3},
				Val: types.NewUint256(4),
			},
		}
		suite.Require().Equal(expected, res.Storage)
	})

	suite.Run("proof", func() {
		tx, err := suite.debugApi.db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		shardId := suite.smcAddr.ShardId()
		accessor := suite.debugApi.accessor.Access(tx, shardId).GetBlock()
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
