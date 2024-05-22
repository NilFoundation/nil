package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteEthBlock struct {
	suite.Suite
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
}

func (suite *SuiteEthBlock) SetupSuite() {
	shardId := types.MasterShardId
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)

	suite.lastBlockHash = common.EmptyHash
	for i := range types.BlockNumber(2) {
		es, err := execution.NewExecutionState(tx, shardId, suite.lastBlockHash)
		suite.Require().NoError(err)

		blockHash, err := es.Commit(i)
		suite.Require().NoError(err)
		suite.lastBlockHash = blockHash
	}

	err = tx.Commit()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api = NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, pool, common.NewLogger("Test", false))
}

func (suite *SuiteEthBlock) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthBlock) TestGetBlockByNumber() {
	_, err := suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.LatestExecutedBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.FinalizedBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.SafeBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.PendingBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	data, err := suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.LatestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.HexToHash("0x302e8f577ca23f0e40beb1aac2ea616f001d32ed0c1ca534bd2af576af171154"), data["parentHash"])
	suite.Equal(suite.lastBlockHash, data["hash"])

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.EarliestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.EmptyHash, data["parentHash"])

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.BlockNumber(1), false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.HexToHash("0x302e8f577ca23f0e40beb1aac2ea616f001d32ed0c1ca534bd2af576af171154"), data["parentHash"])
	suite.Equal(suite.lastBlockHash, data["hash"])

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.BlockNumber(2), false)
	suite.Require().NoError(err)
	suite.Require().Nil(data)
}

func (suite *SuiteEthBlock) TestGetBlockByHash() {
	data, err := suite.api.GetBlockByHash(context.Background(), types.MasterShardId, suite.lastBlockHash, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.HexToHash("0x302e8f577ca23f0e40beb1aac2ea616f001d32ed0c1ca534bd2af576af171154"), data["parentHash"])
	suite.Equal(suite.lastBlockHash, data["hash"])
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByHash() {
	blockHash := common.HexToHash("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")
	_, err := suite.api.GetBlockTransactionCountByHash(context.Background(), types.MasterShardId, blockHash)
	suite.Require().EqualError(err, "not implemented")
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByNumber() {
	_, err := suite.api.GetBlockTransactionCountByNumber(context.Background(), types.MasterShardId, transport.LatestBlockNumber)
	suite.Require().EqualError(err, "not implemented")
}

func TestSuiteEthBlock(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthBlock))
}

func TestGetBlockByNumberOnEmptyBase(t *testing.T) {
	t.Parallel()

	var err error
	db, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)

	pool := msgpool.New(msgpool.DefaultConfig)
	require.NotNil(t, pool)

	ctx := context.Background()
	shardId := types.MasterShardId
	api := NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, pool, common.NewLogger("Test", false))

	data, err := api.GetBlockByNumber(ctx, shardId, transport.EarliestBlockNumber, false)
	require.NoError(t, err)
	require.Nil(t, data)

	data, err = api.GetBlockByNumber(ctx, shardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)
	require.Nil(t, data)

	data, err = api.GetBlockByNumber(ctx, shardId, transport.BlockNumber(123), false)
	require.NoError(t, err)
	require.Nil(t, data)
}
