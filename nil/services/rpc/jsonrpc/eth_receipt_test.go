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
	message     *types.Message
	outMessages []*types.Message
}

func (s *SuiteEthReceipt) SetupSuite() {
	ctx := context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.api = NewTestEthAPI(s.T(), ctx, s.db, 2)

	tx, err := s.db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	s.message = types.NewEmptyMessage()
	s.message.To = types.GenerateRandomAddress(types.BaseShardId)
	s.message.Flags = types.NewMessageFlags(1, 5, 7)

	s.receipt = types.Receipt{MsgHash: s.message.Hash(), Logs: []*types.Log{}, OutMsgIndex: 0, OutMsgNum: 2}

	s.outMessages = append(s.outMessages, &types.Message{MessageDigest: types.MessageDigest{Data: []byte{12}}})
	s.outMessages = append(s.outMessages, &types.Message{MessageDigest: types.MessageDigest{Data: []byte{34}}})

	blockHash := writeTestBlock(s.T(), tx, types.BaseShardId, types.BlockNumber(0), []*types.Message{s.message},
		[]*types.Receipt{&s.receipt}, s.outMessages)
	_, err = execution.PostprocessBlock(tx, types.BaseShardId, types.NewValueFromUint64(10), blockHash)
	s.Require().NoError(err)

	err = tx.Commit()
	s.Require().NoError(err)
}

func (s *SuiteEthReceipt) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthReceipt) TestGetMessageReceipt() {
	data, err := s.api.GetInMessageReceipt(context.Background(), s.receipt.MsgHash)
	s.Require().NoError(err)
	s.Require().NotNil(data)

	for i, outMsg := range s.outMessages {
		s.Equal(outMsg.Hash(), data.OutMessages[i])
	}

	s.Equal(s.receipt.MsgHash, data.MsgHash)
	s.Equal(s.receipt.Success, data.Success)
	s.Equal(s.message.Flags, data.Flags)
}

func TestSuiteEthReceipt(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthReceipt))
}
