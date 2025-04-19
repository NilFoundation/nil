package rollupcontract

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/blob"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethereum "github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type WrapperTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	config           WrapperConfig
	ethClient        *EthClientMock
	callContractMock *testaide.CallContractMock
	wrapper          Wrapper
	logger           logging.Logger
}

func TestWrapperSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(WrapperTestSuite))
}

func (s *WrapperTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())
	s.logger = logging.NewLogger("wrapper_test")
	s.config = NewDefaultWrapperConfig()
	abi, err := RollupcontractMetaData.GetAbi()
	s.Require().NoError(err)
	s.callContractMock = testaide.NewCallContractMock(abi)
	s.ethClient = &EthClientMock{
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
		ChainIDFunc:          func(ctx context.Context) (*big.Int, error) { return big.NewInt(1), nil },
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
			// Create a blob transaction similar to what would be returned in a real scenario
			blobSidecar := &ethtypes.BlobTxSidecar{
				Blobs:       []kzg4844.Blob{{}, {}, {}},
				Commitments: []kzg4844.Commitment{{}, {}, {}},
				Proofs:      []kzg4844.Proof{{}, {}, {}},
			}

			blobTx := &ethtypes.BlobTx{
				ChainID:    uint256.MustFromBig(big.NewInt(11155111)), // Sepolia chainID
				Nonce:      123,
				GasTipCap:  uint256.MustFromBig(big.NewInt(123)),
				GasFeeCap:  uint256.MustFromBig(big.NewInt(123)),
				Gas:        123,
				To:         ethcommon.HexToAddress(s.config.ContractAddressHex),
				Value:      uint256.NewInt(0),
				Data:       []byte{1, 2, 3},
				BlobFeeCap: uint256.MustFromBig(big.NewInt(123)),
				BlobHashes: []ethcommon.Hash{
					ethcommon.HexToHash("0x1111"),
					ethcommon.HexToHash("0x2222"),
					ethcommon.HexToHash("0x3333"),
				},
				Sidecar: blobSidecar,
			}

			tx := ethtypes.NewTx(blobTx)
			return tx, false, nil
		},
		SendTransactionFunc: func(ctx context.Context, tx *ethtypes.Transaction) error {
			return nil
		},
	}

	wrapper, err := NewWrapperWithEthClient(s.ctx, s.config, s.ethClient, s.logger)
	s.Require().NoError(err)
	s.wrapper = wrapper
}

func (s *WrapperTestSuite) SetupTest() {
	s.ethClient.ResetCalls()
	s.callContractMock.Reset()
}

func (s *WrapperTestSuite) TearDownSuite() {
	s.cancellation()
}

// Test LatestFinalizedStateRoot functionality
func (s *WrapperTestSuite) TestFinalizedBatchIndex() {
	// Setup expected return
	expectedRoot := common.HexToHash("1234")
	s.callContractMock.AddExpectedCall("getLastFinalizedBatchIndex", "42")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", expectedRoot)

	// Call method
	root, err := s.wrapper.LatestFinalizedStateRoot(s.ctx)

	// Assert
	s.Require().NoError(err)
	s.Equal(expectedRoot, root)
	s.Require().NoError(s.callContractMock.EverythingCalled())
}

// Test UpdateState - normal case
func (s *WrapperTestSuite) TestUpdateState_Success() {
	batchIndex := "42"
	oldStateRoot := common.HexToHash("0x1111")
	newStateRoot := common.HexToHash("0x2222")
	validityProof := []byte{1, 2, 3}

	// Mock the required contract calls
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("getLastFinalizedBatchIndex", "41")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", oldStateRoot)
	s.callContractMock.AddExpectedCall("updateState", testaide.NoValue{})

	// Create public data inputs
	publicDataInputs := INilRollupPublicDataInfo{
		L2Tol1Root:    common.HexToHash("1234"),
		MessageCount:  big.NewInt(1),
		L1MessageHash: common.HexToHash("1234"),
	}

	// Call method
	err := s.wrapper.UpdateState(
		s.ctx, batchIndex, types.DataProofs{}, oldStateRoot, newStateRoot, validityProof, publicDataInputs,
	)

	// Assert
	s.Require().NoError(err)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Verify transaction was sent
	s.Require().Len(s.ethClient.SendTransactionCalls(), 1)
}

// Test UpdateState - batch already finalized
func (s *WrapperTestSuite) TestUpdateState_AlreadyFinalized() {
	batchIndex := "42"
	oldStateRoot := common.HexToHash("0x1111")
	newStateRoot := common.HexToHash("0x2222")
	validityProof := []byte{1, 2, 3}

	// Mock the batch finalized check to return true
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)
	s.callContractMock.AddExpectedCall("isBatchFinalized", true)

	// Create public data inputs
	publicDataInputs := INilRollupPublicDataInfo{
		L2Tol1Root:    common.HexToHash("1234"),
		MessageCount:  big.NewInt(1),
		L1MessageHash: common.HexToHash("1234"),
	}

	// Call method
	err := s.wrapper.UpdateState(
		s.ctx, batchIndex, types.DataProofs{}, oldStateRoot, newStateRoot, validityProof, publicDataInputs,
	)

	// Assert
	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrBatchAlreadyFinalized)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Verify no transaction was sent
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Test UpdateState - batch not committed
func (s *WrapperTestSuite) TestUpdateState_NotCommitted() {
	batchIndex := "42"
	oldStateRoot := common.HexToHash("0x1111")
	newStateRoot := common.HexToHash("0x2222")
	validityProof := []byte{1, 2, 3}

	// Mock the batch finalized and committed checks
	s.callContractMock.AddExpectedCall("isBatchFinalized", false)
	s.callContractMock.AddExpectedCall("isBatchCommitted", false)

	// Create public data inputs
	publicDataInputs := INilRollupPublicDataInfo{
		L2Tol1Root:    common.HexToHash("1234"),
		MessageCount:  big.NewInt(1),
		L1MessageHash: common.HexToHash("1234"),
	}

	// Call method
	err := s.wrapper.UpdateState(
		s.ctx, batchIndex, types.DataProofs{}, oldStateRoot, newStateRoot, validityProof, publicDataInputs,
	)

	// Assert
	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrBatchNotCommitted)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Verify no transaction was sent
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Test UpdateState - with empty state root
func (s *WrapperTestSuite) TestUpdateState_EmptyStateRoot() {
	batchIndex := "42"
	emptyStateRoot := common.Hash{} // Empty state root
	validityProof := []byte{1, 2, 3}
	publicDataInputs := INilRollupPublicDataInfo{}

	// Test with empty old state root
	err := s.wrapper.UpdateState(
		s.ctx,
		batchIndex,
		types.DataProofs{},
		emptyStateRoot,
		common.HexToHash("0x2222"),
		validityProof,
		publicDataInputs,
	)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "old state root is empty")

	// Test with empty new state root
	err = s.wrapper.UpdateState(
		s.ctx,
		batchIndex,
		types.DataProofs{},
		common.HexToHash("0x1111"),
		emptyStateRoot,
		validityProof,
		publicDataInputs,
	)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "new state root is empty")
}

// Test CommitBatch - success case
func (s *WrapperTestSuite) TestCommitBatch_Success() {
	batchIndex := "42"
	sidecar := s.getSampleSidecar()

	// Mock the batch committed check
	s.callContractMock.AddExpectedCall("isBatchCommitted", false)
	s.callContractMock.AddExpectedCall("commitBatch", testaide.NoValue{})

	// Call method
	err := s.wrapper.CommitBatch(s.ctx, sidecar, batchIndex)

	// Assert
	s.Require().NoError(err)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Verify transaction was sent
	s.Require().Len(s.ethClient.SendTransactionCalls(), 1)
}

// Test CommitBatch - already committed
func (s *WrapperTestSuite) TestCommitBatch_AlreadyCommitted() {
	batchIndex := "42"
	sidecar := s.getSampleSidecar()

	// Mock the batch committed check to return true
	s.callContractMock.AddExpectedCall("isBatchCommitted", true)

	// Call method
	err := s.wrapper.CommitBatch(s.ctx, sidecar, batchIndex)

	// Assert
	s.Require().Error(err)
	s.Require().ErrorIs(err, ErrBatchAlreadyCommitted)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Verify no transaction was sent
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Test verifyDataProofs method
func (s *WrapperTestSuite) TestVerifyDataProofs() {
	// Setup for test
	testWrapper, ok := s.wrapper.(*wrapperImpl)
	s.Require().True(ok)
	hashes := []ethcommon.Hash{
		ethcommon.HexToHash("0x1111"),
		ethcommon.HexToHash("0x2222"),
	}
	dataProofs := [][]byte{
		{1, 2, 3},
		{4, 5, 6},
	}

	// Mock successful verifications
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})

	// Test successful verification
	err := testWrapper.verifyDataProofs(s.ctx, hashes, dataProofs)
	s.Require().NoError(err)
	s.Require().NoError(s.callContractMock.EverythingCalled())

	// Reset and test verification failure
	s.callContractMock.Reset()
	s.callContractMock.AddExpectedCall("verifyDataProof", errors.New("verification failed"))

	err = testWrapper.verifyDataProofs(s.ctx, hashes[:1], dataProofs[:1])
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "proof verification failed")
	s.Require().NoError(s.callContractMock.EverythingCalled())
}

// Verification failure
func (s *WrapperTestSuite) TestPrepareBlobsVerificationFailed() {
	// failing verification, should lead to `PrepareBlobs` error
	s.callContractMock.AddExpectedCall("verifyDataProof", errors.New("verification failed"))

	blobs := []kzg4844.Blob{{}, {}, {}} // Sample empty blobs
	sidecar, dataProofs, err := s.wrapper.PrepareBlobs(s.ctx, blobs)
	s.Require().Error(err)
	s.Require().Nil(sidecar)
	s.Require().Nil(dataProofs)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}

// Test noop wrapper
func (s *WrapperTestSuite) TestNoopWrapper() {
	logger := logging.NewLogger("noop_wrapper_test")
	noopWrapper := &noopWrapper{logger: logger}

	// Test UpdateState
	err := noopWrapper.UpdateState(
		s.ctx, "42", types.DataProofs{}, common.Hash{}, common.Hash{}, nil, INilRollupPublicDataInfo{},
	)
	s.Require().NoError(err)

	// Test FinalizedBatchIndex
	index, err := noopWrapper.LatestFinalizedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Empty(index)

	// Test CommitBatch
	err = noopWrapper.CommitBatch(s.ctx, nil, "42")
	s.Require().NoError(err)
}

func (s *WrapperTestSuite) getSampleSidecar() *ethtypes.BlobTxSidecar {
	s.T().Helper()

	blobBuilder := blob.NewBuilder()
	blobs, err := blobBuilder.MakeBlobs(bytes.NewReader([]byte("hello, world")), 1)
	s.Require().NoError(err)

	// Mock successful verifications
	s.callContractMock.AddExpectedCall("verifyDataProof", testaide.NoValue{})

	sidecar, _, err := s.wrapper.PrepareBlobs(s.ctx, blobs)
	s.Require().NoError(err)
	return sidecar
}
