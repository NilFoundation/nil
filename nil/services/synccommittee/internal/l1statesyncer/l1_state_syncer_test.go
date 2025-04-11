package l1statesyncer

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	ethereum "github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type L1SyncerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	db               db.DB
	clock            clockwork.Clock
	storage          *storage.BlockStorage
	ethClient        *rollupcontract.EthClientMock
	nilClient        *client.ClientMock
	l1Syncer         *L1StateSyncer
	callContractMock *testaide.CallContractMock
}

func TestL1SyncerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(L1SyncerTestSuite))
}

func (s *L1SyncerTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	logger := logging.NewLogger("l1_syncer_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.clock = testaide.NewTestClock()
	s.storage = storage.NewBlockStorage(s.db, storage.DefaultBlockStorageConfig(), s.clock, metricsHandler, logger)

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
	contractWrapper, err := rollupcontract.NewWrapperWithEthClient(
		s.ctx, rollupcontract.NewDefaultWrapperConfig(), s.ethClient, logger,
	)
	s.Require().NoError(err)
	s.nilClient = &client.ClientMock{}
	s.l1Syncer = NewL1StateSyncer(s.storage, contractWrapper, s.nilClient, logger)
	s.Require().NoError(err)
}

func (s *L1SyncerTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.ethClient.ResetCalls()
	s.callContractMock.Reset()
}

func (s *L1SyncerTestSuite) TearDownSuite() {
	s.cancellation()
}

// Test if storage proved state root is updated from L1
func (s *L1SyncerTestSuite) TestStorageProvedRootUpdate() {
	stateRoot := common.HexToHash("0x1234")
	s.callContractMock.AddExpectedCall("finalizedStateRoots", stateRoot)
	s.callContractMock.AddExpectedCall("getLastFinalizedBatchIndex", "testingFinalizedBatchIndex")
	s.Require().NoError(s.l1Syncer.SyncStoredStateRootWithL1(s.ctx))

	storageProvedStateRoot, err := s.storage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(stateRoot, *storageProvedStateRoot)

	s.Require().NoError(s.callContractMock.EverythingCalled())
	s.Require().Empty(s.ethClient.SendTransactionCalls())
}
