package core

import (
	"context"
	"math/big"
	"testing"

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
	"github.com/stretchr/testify/suite"
)

type ProposerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	params    *ProposerParams
	db        db.DB
	timer     common.Timer
	storage   storage.BlockStorage
	ethClient *rollupcontract.EthClientMock
	proposer  *Proposer
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

	s.timer = testaide.NewTestTimer()
	s.storage = storage.NewBlockStorage(s.db, s.timer, metricsHandler, logger)
	s.params = NewDefaultProposerParams()
	s.ethClient = &rollupcontract.EthClientMock{
		CallContractFunc: func(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
			return []byte{123}, nil
		},
		EstimateGasFunc:     func(ctx context.Context, call ethereum.CallMsg) (uint64, error) { return 123, nil },
		SuggestGasPriceFunc: func(ctx context.Context) (*big.Int, error) { return big.NewInt(123), nil },
		HeaderByNumberFunc:  func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) { return &ethtypes.Header{}, nil },
		PendingCodeAtFunc:   func(ctx context.Context, account ethcommon.Address) ([]byte, error) { return []byte{123}, nil },
		PendingNonceAtFunc:  func(ctx context.Context, account ethcommon.Address) (uint64, error) { return 123, nil },
		ChainIDFunc:         func(ctx context.Context) (*big.Int, error) { return big.NewInt(0), nil },
	}
	s.proposer, err = NewProposer(s.ctx, s.params, s.storage, s.ethClient, metricsHandler, logger)
	s.Require().NoError(err)
}

func (s *ProposerTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.ethClient.ResetCalls()
}

func (s *ProposerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *ProposerTestSuite) TestSendProof() {
	data := testaide.NewProposalData(3, s.timer.NowTime())

	err := s.proposer.sendProof(s.ctx, data)
	s.Require().NoError(err, "failed to send proof")

	s.Require().Len(s.ethClient.SendTransactionCalls(), 1, "wrong number of calls to rpc client")
}
