package jsonrpc

import (
	"context"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
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
	defer tx.Rollback()

	suite.lastBlockHash = common.EmptyHash
	for i := range types.BlockNumber(2) {
		es, err := execution.NewExecutionState(tx, shardId, suite.lastBlockHash, common.NewTestTimer(0))
		suite.Require().NoError(err)

		for j := range int(i) {
			m := &types.Message{Data: types.Code(strconv.Itoa(j))}
			es.AddInMessage(m)

			r := &types.Receipt{MsgHash: m.Hash()}
			es.AddReceipt(r)
		}

		blockHash, err := es.Commit(i)
		suite.Require().NoError(err)
		suite.lastBlockHash = blockHash

		block, err := execution.PostprocessBlock(tx, shardId, blockHash)
		suite.Require().NotNil(block)
		suite.Require().NoError(err)
	}

	err = tx.Commit()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api, err = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, []msgpool.Pool{pool}, common.NewLogger("Test"))
	suite.Require().NoError(err)
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
	suite.Equal(suite.lastBlockHash, data.Hash)

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.EarliestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.EmptyHash, data.ParentHash)

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.BlockNumber(1), false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(suite.lastBlockHash, data.Hash)

	data, err = suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.BlockNumber(100500), false)
	suite.Require().NoError(err)
	suite.Require().Nil(data)
}

func (suite *SuiteEthBlock) TestGetBlockByHash() {
	data, err := suite.api.GetBlockByHash(context.Background(), types.MasterShardId, suite.lastBlockHash, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(suite.lastBlockHash, data.Hash)
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByHash() {
	blockHash := common.HexToHash("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")
	res, err := suite.api.GetBlockTransactionCountByHash(context.Background(), types.MasterShardId, blockHash)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(0), *res)

	res, err = suite.api.GetBlockTransactionCountByHash(context.Background(), types.MasterShardId, suite.lastBlockHash)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(1), *res)
}

func (suite *SuiteEthBlock) TestGetBlockContent() {
	resNoFullTx, err := suite.api.GetBlockByHash(context.Background(), types.MasterShardId, suite.lastBlockHash, false)
	suite.Require().NoError(err)
	suite.Len(resNoFullTx.Messages, 1)

	resFullTx, err := suite.api.GetBlockByHash(context.Background(), types.MasterShardId, suite.lastBlockHash, true)
	suite.Require().NoError(err)
	suite.Len(resFullTx.Messages, 1)

	for i, msgAny := range resFullTx.Messages {
		msg, ok := msgAny.(*RPCInMessage)
		suite.Require().True(ok)

		msgHash, ok := resNoFullTx.Messages[i].(common.Hash)
		suite.Require().True(ok)

		suite.Equal(msgHash, msg.Hash)
	}
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByNumber() {
	res, err := suite.api.GetBlockTransactionCountByNumber(context.Background(), types.MasterShardId, 0)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(0), *res)

	res, err = suite.api.GetBlockTransactionCountByNumber(context.Background(), types.MasterShardId, transport.LatestBlockNumber)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(1), *res)

	res, err = suite.api.GetBlockTransactionCountByNumber(context.Background(), types.MasterShardId, 100500)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(0), *res)
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
	api, err := NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, []msgpool.Pool{pool}, common.NewLogger("Test"))
	require.NoError(t, err)

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
