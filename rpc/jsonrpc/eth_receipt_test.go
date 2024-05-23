package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/suite"
)

type SuiteEthReceipt struct {
	suite.Suite
	db      db.DB
	api     *APIImpl
	receipt types.Receipt
}

func (suite *SuiteEthReceipt) SetupSuite() {
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api = NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, pool, common.NewLogger("Test", false))

	tx, err := suite.db.CreateRwTx(ctx)
	defer tx.Rollback()
	suite.Require().NoError(err)

	msgHash := common.EmptyHash
	suite.receipt = types.Receipt{MsgIndex: 100500, MsgHash: msgHash, Logs: []*types.Log{}}

	suite.Require().NoError(db.WriteReceipt(tx, types.MasterShardId, &suite.receipt, msgHash))

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteEthReceipt) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthReceipt) TestGetMessageReceipt() {
	data, err := suite.api.GetMessageReceipt(context.Background(), types.MasterShardId, suite.receipt.MsgHash)
	suite.Require().NoError(err)
	suite.Equal(suite.receipt, *data)
}

func TestSuiteEthReceipt(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthReceipt))
}
