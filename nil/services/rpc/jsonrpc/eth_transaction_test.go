package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteEthTransaction struct {
	suite.Suite
	ctx            context.Context
	db             db.DB
	api            *APIImpl
	lastBlockHash  common.Hash
	transaction    *types.Transaction
	transactionRaw []byte
}

var (
	unknownBlockHash = common.HexToHash("0x0001398db0189885e7cbf70586eeefb9aec472d7216c821866d9254f14269f67")
	unknownTxnHash   = unknownBlockHash
)

func (s *SuiteEthTransaction) SetupSuite() {
	s.ctx = context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.api = NewTestEthAPI(s.T(), s.ctx, s.db, 2)

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	s.transaction = types.NewEmptyTransaction()
	s.transaction.Data = []byte("data")
	s.transaction.To = types.GenerateRandomAddress(types.BaseShardId)
	receipt := types.Receipt{TxnHash: s.transaction.Hash()}

	blockRes := writeTestBlock(
		s.T(),
		tx,
		types.BaseShardId,
		types.BlockNumber(0),
		[]*types.Transaction{s.transaction},
		[]*types.Receipt{&receipt},
		[]*types.Transaction{})
	err = execution.PostprocessBlock(tx, types.BaseShardId, blockRes, execution.ModeVerify)
	s.Require().NoError(err)
	s.lastBlockHash = blockRes.BlockHash

	err = tx.Commit()
	s.Require().NoError(err)

	s.transactionRaw, err = s.transaction.MarshalSSZ()
	s.Require().NoError(err)
}

func (s *SuiteEthTransaction) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthTransaction) TestGetTransactionByHash() {
	data, err := s.api.GetInTransactionByHash(s.ctx, s.transaction.Hash())
	s.Require().NoError(err)
	s.Equal(s.transaction.Hash(), data.Hash)
	s.EqualValues([]byte("data"), data.Data)

	rawData, err := s.api.GetRawInTransactionByHash(s.ctx, s.transaction.Hash())
	s.Require().NoError(err)
	s.Equal(s.transactionRaw, []byte(rawData))

	_, err = s.api.GetInTransactionByHash(s.ctx, unknownTxnHash)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func (s *SuiteEthTransaction) TestGetTransactionBlockNumberAndIndex() {
	data, err := s.api.GetInTransactionByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.transaction.Hash(), data.Hash)

	rawData, err := s.api.GetRawInTransactionByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 0)
	s.Require().NoError(err)
	s.Equal(s.transactionRaw, []byte(rawData))

	_, err = s.api.GetInTransactionByBlockNumberAndIndex(s.ctx, types.BaseShardId, 0, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetInTransactionByBlockNumberAndIndex(s.ctx, types.BaseShardId, 100500, 0)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetRawInTransactionByBlockNumberAndIndex(s.ctx, types.BaseShardId, 100500, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func (s *SuiteEthTransaction) TestGetTransactionBlockHashAndIndex() {
	data, err := s.api.GetInTransactionByBlockHashAndIndex(s.ctx, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.transaction.Hash(), data.Hash)

	rawData, err := s.api.GetRawInTransactionByBlockHashAndIndex(s.ctx, s.lastBlockHash, 0)
	s.Require().NoError(err)
	s.Equal(s.transactionRaw, []byte(rawData))

	_, err = s.api.GetInTransactionByBlockHashAndIndex(s.ctx, s.lastBlockHash, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetInTransactionByBlockHashAndIndex(s.ctx, unknownBlockHash, 0)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)

	_, err = s.api.GetRawInTransactionByBlockHashAndIndex(s.ctx, unknownBlockHash, 100500)
	s.Require().ErrorIs(err, db.ErrKeyNotFound)
}

func TestSuiteEthTransaction(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthTransaction))
}
