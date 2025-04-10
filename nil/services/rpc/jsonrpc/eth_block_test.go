package jsonrpc

import (
	"context"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const shardId = types.MainShardId

type SuiteEthBlock struct {
	suite.Suite

	ctx           context.Context
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
}

func (suite *SuiteEthBlock) SetupSuite() {
	suite.ctx = context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.lastBlockHash = execution.GenerateZeroState(suite.T(), types.MainShardId, suite.db).Hash(types.MainShardId)
	for i := 1; i < int(types.BlockNumber(2)); i++ {
		txns := make([]*types.Transaction, 0, i)
		for j := range i {
			txn := types.NewEmptyTransaction()
			txn.Data = types.Code(strconv.FormatUint(uint64(j), 10))
			txns = append(txns, txn)
		}
		suite.lastBlockHash = execution.GenerateBlockFromTransactionsWithoutExecution(suite.T(),
			shardId, types.BlockNumber(i), suite.lastBlockHash, suite.db, txns...)
	}

	suite.api = NewTestEthAPI(suite.ctx, suite.T(), suite.db, 1)
}

func (suite *SuiteEthBlock) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthBlock) TestGetBlockByNumber() {
	_, err := suite.api.GetBlockByNumber(suite.ctx, shardId, transport.LatestExecutedBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.FinalizedBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.SafeBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	_, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.PendingBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	data, err := suite.api.GetBlockByNumber(suite.ctx, shardId, transport.LatestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(suite.lastBlockHash, data.Hash)

	data, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.EarliestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.EmptyHash, data.ParentHash)

	data, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.BlockNumber(1), false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(suite.lastBlockHash, data.Hash)

	_, err = suite.api.GetBlockByNumber(suite.ctx, shardId, transport.BlockNumber(100500), false)
	suite.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func (suite *SuiteEthBlock) TestGetBlockByHash() {
	data, err := suite.api.GetBlockByHash(suite.ctx, suite.lastBlockHash, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(suite.lastBlockHash, data.Hash)
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByHash() {
	blockHash := common.HexToHash("0x0000117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")
	_, err := suite.api.GetBlockTransactionCountByHash(suite.ctx, blockHash)
	suite.Require().ErrorIs(err, db.ErrKeyNotFound)

	res, err := suite.api.GetBlockTransactionCountByHash(suite.ctx, suite.lastBlockHash)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(1), res)
}

func (suite *SuiteEthBlock) TestGetBlockContent() {
	resNoFullTx, err := suite.api.GetBlockByHash(suite.ctx, suite.lastBlockHash, false)
	suite.Require().NoError(err)
	suite.Len(resNoFullTx.TransactionHashes, 1)
	suite.Empty(resNoFullTx.Transactions)

	resFullTx, err := suite.api.GetBlockByHash(suite.ctx, suite.lastBlockHash, true)
	suite.Require().NoError(err)
	suite.Len(resFullTx.Transactions, 1)
	suite.Empty(resFullTx.TransactionHashes)

	for i, txn := range resFullTx.Transactions {
		suite.Equal(resNoFullTx.TransactionHashes[i], txn.Hash)
	}
}

func (suite *SuiteEthBlock) TestGetBlockTransactionCountByNumber() {
	res, err := suite.api.GetBlockTransactionCountByNumber(suite.ctx, shardId, 0)
	suite.Require().NoError(err)
	suite.Require().Zero(res)

	res, err = suite.api.GetBlockTransactionCountByNumber(suite.ctx, shardId, transport.LatestBlockNumber)
	suite.Require().NoError(err)
	suite.Require().Equal(hexutil.Uint(1), res)

	_, err = suite.api.GetBlockTransactionCountByNumber(suite.ctx, shardId, 100500)
	suite.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func TestSuiteEthBlock(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthBlock))
}

func TestGetBlockByNumberOnEmptyBase(t *testing.T) {
	t.Parallel()

	var err error
	d, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)

	ctx := t.Context()
	api := NewTestEthAPI(ctx, t, d, 1)

	_, err = api.GetBlockByNumber(ctx, shardId, transport.EarliestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	_, err = api.GetBlockByNumber(ctx, shardId, transport.LatestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)

	_, err = api.GetBlockByNumber(ctx, shardId, transport.BlockNumber(123), false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
