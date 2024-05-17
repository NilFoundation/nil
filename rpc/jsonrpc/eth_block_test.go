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
	"github.com/stretchr/testify/suite"
)

type SuiteEthBlock struct {
	suite.Suite
	db        db.DB
	api       *APIImpl
	blockHash common.Hash
}

func (suite *SuiteEthBlock) SetupSuite() {
	shardId := types.MasterShardId
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)

	es, err := execution.NewExecutionState(tx, shardId, common.EmptyHash)
	suite.Require().NoError(err)

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)
	suite.blockHash = blockHash

	err = tx.Put(db.LastBlockTable, shardId.Bytes(), blockHash.Bytes())
	suite.Require().NoError(err)

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
	_, err := suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.EarliestBlockNumber, false)
	suite.Require().EqualError(err, "not implemented")

	data, err := suite.api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.LatestBlockNumber, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.EmptyHash, data["parentHash"])
	suite.Equal(suite.blockHash, data["hash"])
}

func (suite *SuiteEthBlock) TestGetBlockByHash() {
	data, err := suite.api.GetBlockByHash(context.Background(), types.MasterShardId, suite.blockHash, false)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)
	suite.Equal(common.EmptyHash, data["parentHash"])
	suite.Equal(suite.blockHash, data["hash"])
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
