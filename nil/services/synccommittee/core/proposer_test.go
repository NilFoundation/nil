package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethereum "github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type ProposerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	params           ProposerParams
	db               db.DB
	clock            clockwork.Clock
	storage          *storage.BlockStorage
	ethClient        *rollupcontract.EthClientMock
	rpcClientMock    *client.ClientMock
	proposer         *proposer
	testData         *types.ProposalData
	callContractMock *callContractMock
}

type callContractMock struct {
	methodsReturnValue map[string][][]interface{}
}

func newCallContractMock() *callContractMock {
	callContractMock := callContractMock{}
	callContractMock.Reset()
	return &callContractMock
}

func (c *callContractMock) Reset() {
	c.methodsReturnValue = make(map[string][][]interface{})
}

type noValue struct{}

func (c *callContractMock) AddExpectedCall(methodName string, returnValues ...interface{}) {
	c.methodsReturnValue[methodName] = append(c.methodsReturnValue[methodName], returnValues)
}

func (c *callContractMock) CallContract(
	ctx context.Context,
	call ethereum.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	abi, err := rollupcontract.RollupcontractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	methodId := call.Data[:4]
	method, err := abi.MethodById(methodId)
	if err != nil {
		return nil, err
	}

	returnValuesSlice, ok := c.methodsReturnValue[method.Name]
	if !ok {
		return nil, errors.New("method not mocked")
	}

	if len(returnValuesSlice) == 0 {
		return nil, errors.New("not enough return values for method")
	}
	returnValues := returnValuesSlice[0]
	c.methodsReturnValue[method.Name] = returnValuesSlice[1:]

	if len(returnValues) == 1 {
		if _, ok := returnValues[0].(noValue); ok {
			// If it's noValue, call Pack with no arguments
			return method.Outputs.Pack()
		}
	}

	return method.Outputs.Pack(returnValues...)
}

func (c *callContractMock) EverythingCalled() error {
	for methodName, returnValues := range c.methodsReturnValue {
		if len(returnValues) != 0 {
			return fmt.Errorf("not all calls were executed for %s", methodName)
		}
	}
	return nil
}

func TestProposerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ProposerTestSuite))
}

func (s *ProposerTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	logger := logging.NewLogger("proposer_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.clock = testaide.NewTestClock()
	s.storage = storage.NewBlockStorage(s.db, storage.DefaultBlockStorageConfig(), s.clock, metricsHandler, logger)
	s.params = NewDefaultProposerParams()
	s.testData = testaide.NewProposalData(3, s.clock.Now())
	s.callContractMock = newCallContractMock()
	s.ethClient = &rollupcontract.EthClientMock{
		CallContractFunc:    s.callContractMock.CallContract,
		EstimateGasFunc:     func(ctx context.Context, call ethereum.CallMsg) (uint64, error) { return 123, nil },
		SuggestGasPriceFunc: func(ctx context.Context) (*big.Int, error) { return big.NewInt(123), nil },
		HeaderByNumberFunc: func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
			excessBlobGas := uint64(123)
			return &ethtypes.Header{BaseFee: big.NewInt(123), ExcessBlobGas: &excessBlobGas}, nil
		},
		PendingCodeAtFunc: func(ctx context.Context, account ethcommon.Address) ([]byte, error) {
			return []byte{123}, nil
		},
		PendingNonceAtFunc:   func(ctx context.Context, account ethcommon.Address) (uint64, error) { return 123, nil },
		ChainIDFunc:          func(ctx context.Context) (*big.Int, error) { return big.NewInt(0), nil },
		SuggestGasTipCapFunc: func(ctx context.Context) (*big.Int, error) { return big.NewInt(123), nil },
		CodeAtFunc: func(ctx context.Context, contract ethcommon.Address, blockNumber *big.Int) ([]byte, error) {
			return []byte{123}, nil
		},
		TransactionReceiptFunc: func(ctx context.Context, txHash ethcommon.Hash) (*ethtypes.Receipt, error) {
			return &ethtypes.Receipt{Status: ethtypes.ReceiptStatusSuccessful}, nil
		},
	}
	s.rpcClientMock = &client.ClientMock{}
	s.proposer, err = NewProposer(s.ctx, s.params, s.storage, s.ethClient, s.rpcClientMock, metricsHandler, logger)
	s.Require().NoError(err)
}

func (s *ProposerTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.ethClient.ResetCalls()
	s.callContractMock.Reset()
}

func (s *ProposerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *ProposerTestSuite) TestSendProof() {
	// Calls inside CommitBatch
	s.callContractMock.AddExpectedCall("isBatchCommitted", false)
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)

	err := s.proposer.sendProof(s.ctx, s.testData)
	s.Require().NoError(err, "failed to send proof")

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Len(s.ethClient.SendTransactionCalls(), 2, "wrong number of calls to rpc client")
}

// Only UpdateState tx should be created
func (s *ProposerTestSuite) TestSendProofCommitedBatch() {
	// Calls inside CommitBatch
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)

	err := s.proposer.sendProof(s.ctx, s.testData)
	s.Require().NoError(err, "failed to send proof")

	s.Require().Len(s.ethClient.SendTransactionCalls(), 1, "wrong number of calls to rpc client")
}

// No tx should be created
func (s *ProposerTestSuite) TestSendProofFinalizedBatch() {
	// Calls inside CommitBatch
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", noValue{})
	s.callContractMock.AddExpectedCall("isBatchFinalized", true)

	err := s.proposer.sendProof(s.ctx, s.testData)
	s.Require().NoError(err, "failed to send proof")

	s.Require().Empty(s.ethClient.SendTransactionCalls(), "no tx should be created")
}
