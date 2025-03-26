package core

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethereum "github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type ProposerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	params           ProposerConfig
	db               db.DB
	clock            clockwork.Clock
	storage          *storage.BlockStorage
	ethClient        *rollupcontract.EthClientMock
	rpcClientMock    *client.ClientMock
	proposer         *proposer
	testData         *types.ProposalData
	callContractMock *testaide.CallContractMock
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
	s.params = NewDefaultProposerConfig()
	s.testData = testaide.NewProposalData(3, s.clock.Now())

	abi, err := rollupcontract.RollupcontractMetaData.GetAbi()
	s.Require().NoError(err)
	s.callContractMock = testaide.NewCallContractMock(abi)
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
		FilterLogsFunc: func(ctx context.Context, q ethereum.FilterQuery) ([]ethtypes.Log, error) {
			return []ethtypes.Log{{
				Topics: []ethcommon.Hash{q.Topics[0][0], q.Topics[1][0]},
				TxHash: ethcommon.HexToHash("0x12345"),
			}}, nil
		},
		TransactionByHashFunc: func(ctx context.Context, hash ethcommon.Hash) (*ethtypes.Transaction, bool, error) {
			txInTest := ethtypes.NewTx(&ethtypes.BlobTx{
				Sidecar: &ethtypes.BlobTxSidecar{
					// only number of elements matters here, validation is mocked
					Blobs:       []kzg4844.Blob{{}, {}, {}},
					Commitments: []kzg4844.Commitment{{}, {}, {}},
				},
			})
			return txInTest, false, nil
		},
	}
	s.rpcClientMock = &client.ClientMock{}
	contractWrapper, err := rollupcontract.NewWrapperWithEthClient(
		s.ctx, rollupcontract.NewDefaultWrapperConfig(), s.ethClient, logger,
	)
	s.Require().NoError(err)
	s.proposer, err = NewProposer(s.params, s.storage, contractWrapper, s.rpcClientMock, metricsHandler, logger)
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

// Normal execution
func (s *ProposerTestSuite) TestSendProofCommitedBatch() {
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)
	// one verification per blob
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})

	err := s.proposer.updateState(s.ctx, s.testData)
	s.Require().NoError(err, "failed to send proof")

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Len(s.ethClient.SendTransactionCalls(), 1, "wrong number of calls to rpc client")
}

// Batch not committed, should fail
func (s *ProposerTestSuite) TestSendProofNotCommitedBatch() {
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", false)

	err := s.proposer.updateState(s.ctx, s.testData)
	s.Require().ErrorIs(err, rollupcontract.ErrBatchNotCommitted)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Batch already finalized, should fail
func (s *ProposerTestSuite) TestSendProofFinalizedBatch() {
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("isBatchFinalized", true)

	err := s.proposer.updateState(s.ctx, s.testData)
	s.Require().ErrorIs(err, rollupcontract.ErrBatchAlreadyFinalized)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls(), "no tx should be created")
}

// Verification failure
func (s *ProposerTestSuite) TestSendProofVerificationFailed() {
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)
	// failing verification, should lead to `updateState` error
	s.callContractMock.AddExpectedCall("verifyDataProof", errors.New("verification failed"))

	err := s.proposer.updateState(s.ctx, s.testData)
	s.Require().Error(err)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Test if proposal data is removed from the storage on success
func (s *ProposerTestSuite) TestStorageProposalDataRemoved() {
	// Calls inside UpdateState
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)
	// one verification per blob
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})

	s.Require().NoError(s.storage.SetBlockBatch(
		s.ctx,
		&types.BlockBatch{Id: s.testData.BatchId, MainShardBlock: &jsonrpc.RPCBlock{Hash: common.HexToHash("0x1")}},
	))
	s.Require().NoError(s.storage.SetBatchAsProved(s.ctx, s.testData.BatchId))
	s.Require().NoError(s.storage.SetProvedStateRoot(s.ctx, s.testData.OldProvedStateRoot))

	err := s.proposer.updateStateIfReady(s.ctx)
	s.Require().NoError(err)

	// after `SetBatchAsProposed` call inside `updateStateIfReady` there should be no new proposal data
	data, err := s.storage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(data)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Len(s.ethClient.SendTransactionCalls(), 1)
}

// Test if storage proved state root is updated from L1
func (s *ProposerTestSuite) TestStorageProvedRootUpdate() {
	// Calls inside FinalizedBatchIndex
	s.callContractMock.AddExpectedCall("finalizedStateRoots", s.testData.OldProvedStateRoot)
	s.callContractMock.AddExpectedCall("lastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	err := s.proposer.updateStoredStateRootFromL1(s.ctx)
	s.Require().NoError(err)

	storageProvedStateRoot, err := s.storage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(s.testData.OldProvedStateRoot, *storageProvedStateRoot)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}
