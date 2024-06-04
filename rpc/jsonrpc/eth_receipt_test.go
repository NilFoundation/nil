package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
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

	suite.api = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, []msgpool.Pool{pool}, common.NewLogger("Test"))

	tx, err := suite.db.CreateRwTx(ctx)
	defer tx.Rollback()
	suite.Require().NoError(err)

	message := types.Message{Data: []byte{}}
	msgHash := message.Hash()
	suite.receipt = types.Receipt{MsgIndex: 0, MsgHash: msgHash, Logs: []*types.Log{}}

	blockHash := writeTestBlock(suite.T(), tx, types.MasterShardId, types.BlockNumber(0), []*types.Message{&message}, []*types.Receipt{&suite.receipt})
	_, err = execution.PostprocessBlock(tx, types.MasterShardId, blockHash)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteEthReceipt) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthReceipt) TestGetMessageReceipt() {
	data, err := suite.api.GetInMessageReceipt(context.Background(), types.MasterShardId, suite.receipt.MsgHash)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)

	suite.Equal(suite.receipt.MsgHash, data.MsgHash)
	suite.Equal(suite.receipt.Success, data.Success)
}

func TestSuiteEthReceipt(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthReceipt))
}
