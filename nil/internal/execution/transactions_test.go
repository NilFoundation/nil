package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"
)

type TransactionsSuite struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (s *TransactionsSuite) SetupSuite() {
	s.ctx = context.Background()
}

func (s *TransactionsSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *TransactionsSuite) TearDownTest() {
	s.db.Close()
}

func (s *TransactionsSuite) TestValidateExternalTransaction() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(es)
	es.BaseFee = types.DefaultGasPrice
	es.GasPrice = es.BaseFee

	validate := func(txn *types.Transaction) types.ExecError {
		res := ValidateExternalTransaction(es, txn)
		s.Require().False(res.IsFatal())
		if res.Failed() {
			return res.Error
		}
		return nil
	}

	s.Run("Deploy", func() {
		code := types.Code("some-code")
		txn := types.NewEmptyTransaction()
		txn.Flags = types.NewTransactionFlags(types.TransactionFlagDeploy)
		txn.To = types.GenerateRandomAddress(types.BaseShardId)
		txn.Data = types.BuildDeployPayload(code, common.EmptyHash).Bytes()
		txn.MaxFeePerGas = types.MaxFeePerGasDefault

		s.Run("NoAccount", func() {
			s.Require().Equal(types.ErrorDestinationContractDoesNotExist, validate(txn).Code())

			s.Require().NoError(es.CreateAccount(txn.To))
		})

		s.Run("IncorrectAddress", func() {
			s.Require().Equal(types.ErrorIncorrectDeploymentAddress, validate(txn).Code())

			txn.To = types.CreateAddress(types.BaseShardId, *types.ParseDeployPayload(txn.Data))
			s.Require().NoError(es.CreateAccount(txn.To))
		})

		s.Run("Ok", func() {
			s.Require().NoError(validate(txn))
		})

		s.Run("ContractAlreadyExists", func() {
			s.Require().NoError(es.SetCode(txn.To, code))

			s.Require().Equal(types.ErrorContractAlreadyExists, validate(txn).Code())
		})
	})

	s.Run("Execution", func() {
		txn := types.NewEmptyTransaction()
		txn.To = types.GenerateRandomAddress(types.BaseShardId)
		txn.Data = []byte("hello")
		txn.MaxFeePerGas = types.MaxFeePerGasDefault

		s.Run("NoAccount", func() {
			s.Require().Equal(types.ErrorDestinationContractDoesNotExist, validate(txn).Code())

			s.Require().NoError(es.CreateAccount(txn.To))
		})

		s.Run("NoContract", func() {
			s.Require().Equal(types.ErrorContractDoesNotExist, validate(txn).Code())

			// contract that always returns "true",
			// so verifies any transaction
			s.Require().NoError(es.SetCode(txn.To, ethcommon.FromHex("600160005260206000f3")))
		})

		s.Run("NoBalance", func() {
			s.Require().Equal(types.ErrorInsufficientBalance, validate(txn).Code())

			s.Require().NoError(es.SetBalance(txn.To, types.NewValueFromUint64(10_000_000_000_000_000)))
		})

		s.Run("Ok", func() {
			s.Require().NoError(validate(txn))
		})

		// todo: fail signature verification

		s.Run("InvalidChain", func() {
			txn.ChainId = 100500
			s.Require().Equal(types.ErrorInvalidChainId, validate(txn).Code())
		})

		s.Run("SeqnoGap", func() {
			txn.ChainId = types.DefaultChainId
			txn.Seqno = 100
			s.Require().Equal(types.ErrorSeqnoGap, validate(txn).Code())
		})
	})
}

func (s *TransactionsSuite) TestValidateDeployTransaction() {
	txn := types.NewEmptyTransaction()
	txn.Data = types.Code("no-salt")
	txn.MaxFeePerGas = types.MaxFeePerGasDefault

	// data too short
	s.Require().Equal(types.ErrorInvalidPayload, types.GetErrorCode(ValidateDeployTransaction(txn)))

	// Deploy to the main shard from base shard - FAIL
	data := types.BuildDeployPayload([]byte("some-code"), common.EmptyHash)
	txn.To = types.CreateAddress(types.MainShardId, data)
	txn.Data = data.Bytes()
	s.Require().Equal(types.ErrorDeployToMainShard, types.GetErrorCode(ValidateDeployTransaction(txn)))

	// Deploy to base shard
	txn.To = types.CreateAddress(types.BaseShardId, data)
	s.Require().NoError(ValidateDeployTransaction(txn))
}

func TestTransactions(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TransactionsSuite))
}
