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

type SuiteEthMessage struct {
	suite.Suite
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
	message       types.Message
	messageRaw    []byte
}

var unknownBlockHash = common.HexToHash("0x00eb398db0189885e7cbf70586eeefb9aec472d7216c821866d9254f14269f67")

func (suite *SuiteEthMessage) SetupSuite() {
	ctx := context.Background()

	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	suite.Require().NotNil(pool)

	suite.api, err = NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), suite.db, []msgpool.Pool{pool, pool}, common.NewLogger("Test"))
	suite.Require().NoError(err)

	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	suite.message = types.Message{Data: []byte("data")}
	receipt := types.Receipt{MsgHash: suite.message.Hash()}

	suite.lastBlockHash = writeTestBlock(
		suite.T(), tx, types.BaseShardId, types.BlockNumber(0), []*types.Message{&suite.message}, []*types.Receipt{&receipt})
	_, err = execution.PostprocessBlock(tx, types.BaseShardId, suite.lastBlockHash)
	suite.Require().NoError(err)

	err = tx.Commit()
	suite.Require().NoError(err)

	suite.messageRaw, err = suite.message.MarshalSSZ()
	suite.Require().NoError(err)
}

func (suite *SuiteEthMessage) TearDownSuite() {
	suite.db.Close()
}

func (s *SuiteEthMessage) TestGetMessageByHash() {
	data, err := s.api.GetInMessageByHash(context.Background(), types.BaseShardId, s.message.Hash())
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)

	rawData, err := s.api.GetRawInMessageByHash(context.Background(), types.BaseShardId, s.message.Hash())
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	data, err = s.api.GetInMessageByHash(context.Background(), types.BaseShardId, unknownBlockHash)
	s.Require().NoError(err)
	s.Nil(data)
}

func (s *SuiteEthMessage) TestGetMessageyBlockNumberAndIndex() {
	data, err := s.api.GetInMessageByBlockNumberAndIndex(context.Background(), types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)

	rawData, err := s.api.GetRawInMessageByBlockNumberAndIndex(context.Background(), types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	data, err = s.api.GetInMessageByBlockNumberAndIndex(context.Background(), types.BaseShardId, 0, 100500)
	s.Require().NoError(err)
	s.Nil(data)

	data, err = s.api.GetInMessageByBlockNumberAndIndex(context.Background(), types.BaseShardId, 100500, 0)
	s.Require().NoError(err)
	s.Nil(data)

	rawData, err = s.api.GetRawInMessageByBlockNumberAndIndex(context.Background(), types.BaseShardId, 100500, 100500)
	s.Require().NoError(err)
	s.Nil(rawData)
}

func (s *SuiteEthMessage) TestGetMessageyBlockHashAndIndex() {
	data, err := s.api.GetInMessageByBlockHashAndIndex(context.Background(), types.BaseShardId, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)

	rawData, err := s.api.GetRawInMessageByBlockHashAndIndex(context.Background(), types.BaseShardId, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	data, err = s.api.GetInMessageByBlockHashAndIndex(context.Background(), types.BaseShardId, s.lastBlockHash, 100500)
	s.Require().NoError(err)
	s.Nil(data)

	data, err = s.api.GetInMessageByBlockHashAndIndex(context.Background(), types.BaseShardId, unknownBlockHash, 0)
	s.Require().NoError(err)
	s.Nil(data)

	rawData, err = s.api.GetRawInMessageByBlockHashAndIndex(context.Background(), types.BaseShardId, unknownBlockHash, 100500)
	s.Require().NoError(err)
	s.Nil(rawData)
}

func TestSuiteEthMessage(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthMessage))
}
