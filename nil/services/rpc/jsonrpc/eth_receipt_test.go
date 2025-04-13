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
	db              db.DB
	api             *APIImpl
	receipt         types.Receipt
	transaction     *types.Transaction
	outTransactions []*types.Transaction
}

func (s *SuiteEthReceipt) SetupSuite() {
	ctx := context.Background()

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.api = NewTestEthAPI(ctx, s.T(), s.db, 2)

	tx, err := s.db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	s.transaction = types.NewEmptyTransaction()
	s.transaction.To = types.GenerateRandomAddress(types.BaseShardId)
	s.transaction.Flags = types.NewTransactionFlags(1, 5, 7)

	s.receipt = types.Receipt{TxnHash: s.transaction.Hash(), Logs: []*types.Log{}, OutTxnIndex: 0, OutTxnNum: 2}

	s.outTransactions = append(
		s.outTransactions,
		&types.Transaction{
			TransactionDigest: types.TransactionDigest{Data: []byte{12}},
			TxId:              0,
		},
		&types.Transaction{
			TransactionDigest: types.TransactionDigest{Data: []byte{34}},
			TxId:              1,
		})

	blockRes := writeTestBlock(s.T(), tx, types.BaseShardId, types.BlockNumber(0), []*types.Transaction{s.transaction},
		[]*types.Receipt{&s.receipt}, s.outTransactions)
	err = execution.PostprocessBlock(tx, types.BaseShardId, blockRes, execution.ModeVerify)
	s.Require().NoError(err)

	err = tx.Commit()
	s.Require().NoError(err)
}

func (s *SuiteEthReceipt) TearDownSuite() {
	s.db.Close()
}

func (s *SuiteEthReceipt) TestGetTransactionReceipt() {
	data, err := s.api.GetInTransactionReceipt(context.Background(), s.receipt.TxnHash)
	s.Require().NoError(err)
	s.Require().NotNil(data)

	for i, outTxn := range s.outTransactions {
		s.Equal(outTxn.Hash(), data.OutTransactions[i])
	}

	s.Equal(s.receipt.TxnHash, data.TxnHash)
	s.Equal(s.receipt.Success, data.Success)
	s.Equal(s.transaction.Flags, data.Flags)
}

func TestSuiteEthReceipt(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthReceipt))
}
