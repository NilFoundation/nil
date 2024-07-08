package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/suite"
)

type SuiteEthMessage struct {
	suite.Suite
	ctx           context.Context
	db            db.DB
	api           *APIImpl
	lastBlockHash common.Hash
	message       types.Message
	messageRaw    []byte
}

var unknownBlockHash = common.HexToHash("0x00eb398db0189885e7cbf70586eeefb9aec472d7216c821866d9254f14269f67")

func (s *SuiteEthMessage) SetupSuite() {
	s.ctx = context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	s.Require().NotNil(pool)

	s.api, err = NewEthAPI(s.ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), s.db, []msgpool.Pool{pool, pool}, logging.NewLogger("Test"))
	s.Require().NoError(err)

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	s.message = types.Message{Data: []byte("data")}
	receipt := types.Receipt{MsgHash: s.message.Hash()}

	s.lastBlockHash = writeTestBlock(
		s.T(), tx, types.BaseShardId, types.BlockNumber(0), []*types.Message{&s.message}, []*types.Receipt{&receipt}, []*types.Message{})
	_, err = execution.PostprocessBlock(tx, types.BaseShardId, types.NewValueFromUint64(10), 0, s.lastBlockHash)
	s.Require().NoError(err)

	err = tx.Commit()
	s.Require().NoError(err)

	s.messageRaw, err = s.message.MarshalSSZ()
	s.Require().NoError(err)
}

func (s *SuiteEthMessage) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthMessage) TestGetMessageByHash() {
	data, err := s.api.GetInMessageByHash(s.ctx, types.BaseShardId, s.message.Hash())
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)
	s.EqualValues([]byte("data"), data.Data)

	rawData, err := s.api.GetRawInMessageByHash(s.ctx, types.BaseShardId, s.message.Hash())
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	_, err = s.api.GetInMessageByHash(s.ctx, types.BaseShardId, unknownBlockHash)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func (s *SuiteEthMessage) TestGetMessageBlockNumberAndIndex() {
	data, err := s.api.GetInMessageByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)

	rawData, err := s.api.GetRawInMessageByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	_, err = s.api.GetInMessageByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetInMessageByBlockNumberAndIndex(s.ctx, types.BaseShardId, 100500, 0)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetRawInMessageByBlockNumberAndIndex(s.ctx, types.BaseShardId, 100500, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func (s *SuiteEthMessage) TestGetMessageBlockHashAndIndex() {
	data, err := s.api.GetInMessageByBlockHashAndIndex(s.ctx, types.BaseShardId, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.message.Hash(), data.Hash)

	rawData, err := s.api.GetRawInMessageByBlockHashAndIndex(s.ctx, types.BaseShardId, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.messageRaw, []byte(rawData))

	_, err = s.api.GetInMessageByBlockHashAndIndex(s.ctx, types.BaseShardId, s.lastBlockHash, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetInMessageByBlockHashAndIndex(s.ctx, types.BaseShardId, unknownBlockHash, 0)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetRawInMessageByBlockHashAndIndex(s.ctx, types.BaseShardId, unknownBlockHash, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func TestSuiteEthMessage(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthMessage))
}
