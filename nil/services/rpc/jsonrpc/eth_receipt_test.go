package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteEthReceipt struct {
	suite.Suite
	db          db.DB
	api         *APIImpl
	receipt     types.Receipt
	outMessages []*types.Message
}

func (suite *SuiteEthReceipt) SetupSuite() {
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.api = NewTestEthAPI(suite.T(), ctx, suite.db, 2)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	message := types.Message{Data: []byte{}, To: types.GenerateRandomAddress(types.BaseShardId)}
	suite.receipt = types.Receipt{MsgHash: message.Hash(), Logs: []*types.Log{}, OutMsgIndex: 0, OutMsgNum: 2}

	suite.outMessages = append(suite.outMessages, &types.Message{Data: []byte{12}})
	suite.outMessages = append(suite.outMessages, &types.Message{Data: []byte{34}})

	blockHash := writeTestBlock(suite.T(), tx, types.BaseShardId, types.BlockNumber(0), []*types.Message{&message},
		[]*types.Receipt{&suite.receipt}, suite.outMessages)
	_, err = execution.PostprocessBlock(tx, types.BaseShardId, types.NewValueFromUint64(10), blockHash)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteEthReceipt) TearDownSuite() {
	suite.db.Close()
}

func (suite *SuiteEthReceipt) TestGetMessageReceipt() {
	data, err := suite.api.GetInMessageReceipt(context.Background(), suite.receipt.MsgHash)
	suite.Require().NoError(err)
	suite.Require().NotNil(data)

	for i, outMsg := range suite.outMessages {
		suite.Equal(outMsg.Hash(), data.OutMessages[i])
	}

	suite.Equal(suite.receipt.MsgHash, data.MsgHash)
	suite.Equal(suite.receipt.Success, data.Success)
}

func TestSuiteEthReceipt(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthReceipt))
}
