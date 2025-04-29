package syncer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type L1SyncerTestSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	logger         logging.Logger
	rpcClient      *client.ClientMock
	rollupContract *rollupcontract.WrapperMock

	db           db.DB
	blockStorage *storage.BlockStorage

	syncer *stateRootSyncer
}

func TestL1SyncerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(L1SyncerTestSuite))
}

func (s *L1SyncerTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = logging.NewLogger("state_root_syncer_test")

	s.rpcClient = &client.ClientMock{}
	s.rollupContract = &rollupcontract.WrapperMock{}

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.blockStorage = s.newTestBlockStorage()

	s.syncer = NewStateRootSyncer(
		fetching.NewFetcher(s.rpcClient, s.logger),
		s.rollupContract,
		s.blockStorage,
		s.logger,
		NewDefaultConfig(),
	)
}

func (s *L1SyncerTestSuite) newTestBlockStorage() *storage.BlockStorage {
	s.T().Helper()

	config := storage.DefaultBlockStorageConfig()
	clock := clockwork.NewRealClock()
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	return storage.NewBlockStorage(s.db, config, clock, metricsHandler, s.logger)
}

func (s *L1SyncerTestSuite) SetupTest() {
	s.reset()
}

func (s *L1SyncerTestSuite) SetupSubTest() {
	s.reset()
}

func (s *L1SyncerTestSuite) reset() {
	s.T().Helper()
	s.syncer.config = NewDefaultConfig()
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database during reset")
	s.rpcClient.ResetCalls()
	s.rollupContract.ResetCalls()
}

func (s *L1SyncerTestSuite) TearDownTest() {
	s.cancel()
}

func (s *L1SyncerTestSuite) Test_EnsureL1StateIsInitialized_L1_Already_Initialized() {
	finalizedRoot := testaide.RandomHash()
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return finalizedRoot, nil
	}

	err := s.syncer.EnsureL1StateIsInitialized(s.ctx)
	s.Require().NoError(err)

	getBlockCalls := s.rpcClient.GetBlockCalls()
	s.Require().Empty(getBlockCalls)
}

func (s *L1SyncerTestSuite) Test_EnsureL1StateIsInitialized_FetchGenesisBlock_Success() {
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return common.EmptyHash, nil
	}
	genesisHash := testaide.RandomHash()
	s.rpcClient.GetBlockFunc = func(
		_ context.Context, shardId coreTypes.ShardId, blockID any, fullTx bool,
	) (*types.Block, error) {
		block := testaide.NewGenesisBlock(coreTypes.MainShardId)
		block.Hash = genesisHash
		return block, nil
	}
	s.rollupContract.SetGenesisStateRootFunc = func(ctx context.Context, hash common.Hash) error {
		return nil
	}

	err := s.syncer.EnsureL1StateIsInitialized(s.ctx)
	s.Require().NoError(err)

	setGenesisRootCalls := s.rollupContract.SetGenesisStateRootCalls()
	s.Require().Len(setGenesisRootCalls, 1)
	s.Require().Equal(genesisHash, setGenesisRootCalls[0].GenesisStateRoot)
}

func (s *L1SyncerTestSuite) Test_EnsureL1StateIsInitialized_GetLatestFinalizedStateRoot_Fails() {
	l1Error := errors.New("something went wrong")
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return common.EmptyHash, l1Error
	}

	err := s.syncer.EnsureL1StateIsInitialized(s.ctx)
	s.Require().ErrorIs(err, l1Error)
}

func (s *L1SyncerTestSuite) Test_EnsureL1StateIsInitialized_FetchGenesisBlock_Fails() {
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return common.EmptyHash, nil
	}

	l2Error := errors.New("something went wrong")

	s.rpcClient.GetBlockFunc = func(
		_ context.Context, shardId coreTypes.ShardId, blockID any, fullTx bool,
	) (*types.Block, error) {
		return nil, l2Error
	}

	err := s.syncer.EnsureL1StateIsInitialized(s.ctx)
	s.Require().ErrorIs(err, l2Error)

	setGenesisRootCalls := s.rollupContract.SetGenesisStateRootCalls()
	s.Require().Empty(setGenesisRootCalls)
}

func (s *L1SyncerTestSuite) Test_EnsureL1StateIsInitialized_SetGenesisStateRoot_Fails() {
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return common.EmptyHash, nil
	}
	genesisHash := testaide.RandomHash()
	s.rpcClient.GetBlockFunc = func(
		_ context.Context, shardId coreTypes.ShardId, blockID any, fullTx bool,
	) (*types.Block, error) {
		block := testaide.NewGenesisBlock(coreTypes.MainShardId)
		block.Hash = genesisHash
		return block, nil
	}

	setGenesisErr := errors.New("failed to set genesis root")

	s.rollupContract.SetGenesisStateRootFunc = func(ctx context.Context, hash common.Hash) error {
		return setGenesisErr
	}

	err := s.syncer.EnsureL1StateIsInitialized(s.ctx)
	s.Require().ErrorIs(err, setGenesisErr)
}

func (s *L1SyncerTestSuite) Test_SyncLatestFinalizedRoot_EmptyLocal_NonEmptyL1() {
	for _, testCase := range s.configTestCases() {
		s.Run(testCase.name, func() {
			s.syncer.config.AlwaysSyncWithL1 = testCase.alwaysSyncWithL1
			s.setupClientMock(testaide.RandomHash(), true)

			latestFinalizedHash := testaide.RandomHash()
			s.setLatestFinalizedHash(latestFinalizedHash)

			err := s.syncer.SyncLatestFinalizedRoot(s.ctx)
			s.Require().NoError(err)

			s.requireLocalRoot(latestFinalizedHash)
		})
	}
}

func (s *L1SyncerTestSuite) Test_SyncLatestFinalizedRoot_EmptyLocal_EmptyL1() {
	for _, testCase := range s.configTestCases() {
		s.Run(testCase.name, func() {
			s.syncer.config.AlwaysSyncWithL1 = testCase.alwaysSyncWithL1

			genesisHash := testaide.RandomHash()
			s.setupClientMock(genesisHash, true)

			s.setLatestFinalizedHash(common.EmptyHash)

			err := s.syncer.SyncLatestFinalizedRoot(s.ctx)
			s.Require().ErrorIs(err, types.ErrL1StateRootNotInitialized)

			storedRoot, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
			s.Require().NoError(err)
			s.Require().Nil(storedRoot)

			getBlockCalls := s.rpcClient.GetBlockCalls()
			s.Empty(getBlockCalls)
		})
	}
}

type nonEmptyLocalTestCase struct {
	name       string
	syncWithL1 bool
	existsOnL2 bool
}

func (s *L1SyncerTestSuite) Test_SyncLatestFinalizedRoot_NonEmptyLocal() {
	testCases := []nonEmptyLocalTestCase{
		{"AlwaysSyncWithL1_False_Exists_On_L2_False", false, false},
		{"AlwaysSyncWithL1_False_Exists_On_L2_True", false, true},
		{"AlwaysSyncWithL1_True_Exists_On_L2_False", true, false},
		{"AlwaysSyncWithL1_True_Exists_On_L2_True", true, true},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.runNonEmptyLocalTestCase(testCase)
		})
	}
}

func (s *L1SyncerTestSuite) runNonEmptyLocalTestCase(testCase nonEmptyLocalTestCase) {
	s.T().Helper()
	s.syncer.config.AlwaysSyncWithL1 = testCase.syncWithL1
	s.setupClientMock(testaide.RandomHash(), testCase.existsOnL2)

	latestFinalizedHash := testaide.RandomHash()
	s.setLatestFinalizedHash(latestFinalizedHash)

	localHash := testaide.RandomHash()
	err := s.blockStorage.SetProvedStateRoot(s.ctx, localHash)
	s.Require().NoError(err)

	err = s.syncer.SyncLatestFinalizedRoot(s.ctx)

	switch {
	case testCase.syncWithL1 && testCase.existsOnL2:
		s.Require().NoError(err)
		s.requireLocalRoot(latestFinalizedHash)

	case testCase.syncWithL1 && !testCase.existsOnL2:
		s.Require().ErrorIs(err, types.ErrStateRootNotSynced)
		s.requireLocalRoot(localHash)

	case !testCase.syncWithL1 && testCase.existsOnL2:
		s.Require().NoError(err)
		s.requireLocalRoot(localHash)

	case !testCase.syncWithL1 && !testCase.existsOnL2:
		s.Require().ErrorIs(err, types.ErrStateRootNotSynced)
		s.requireLocalRoot(localHash)
	}
}

func (s *L1SyncerTestSuite) requireLocalRoot(latestFinalizedHash common.Hash) {
	s.T().Helper()
	storedRoot, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(storedRoot)
	s.Require().Equal(latestFinalizedHash, *storedRoot)
}

func (s *L1SyncerTestSuite) Test_SyncLatestFinalizedRoot_L2_Returns_Nil() {
	for _, testCase := range s.configTestCases() {
		s.Run(testCase.name, func() {
			s.syncer.config.AlwaysSyncWithL1 = testCase.alwaysSyncWithL1

			s.rpcClient.GetBlockFunc = func(
				_ context.Context, shardId coreTypes.ShardId, blockId any, fullTx bool,
			) (*types.Block, error) {
				return nil, nil
			}

			s.setLatestFinalizedHash(common.EmptyHash)

			err := s.syncer.SyncLatestFinalizedRoot(s.ctx)
			s.Require().ErrorIs(err, types.ErrStateRootNotSynced)

			storedRoot, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
			s.Require().NoError(err)
			s.Require().Nil(storedRoot)
		})
	}
}

func (s *L1SyncerTestSuite) Test_SyncLatestFinalizedRoot_L1_Returns_Error() {
	for _, testCase := range s.configTestCases() {
		s.Run(testCase.name, func() {
			s.syncer.config.AlwaysSyncWithL1 = testCase.alwaysSyncWithL1
			s.setupClientMock(testaide.RandomHash(), true)

			s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
				return common.EmptyHash, errors.New("something went wrong")
			}

			err := s.syncer.SyncLatestFinalizedRoot(s.ctx)
			s.Require().ErrorIs(err, types.ErrStateRootNotSynced)

			storedRoot, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
			s.Require().NoError(err)
			s.Require().Nil(storedRoot)
		})
	}
}

func (s *L1SyncerTestSuite) configTestCases() []struct {
	name             string
	alwaysSyncWithL1 bool
} {
	s.T().Helper()
	return []struct {
		name             string
		alwaysSyncWithL1 bool
	}{
		{"AlwaysSyncWithL1_False", false},
		{"AlwaysSyncWithL1_True", true},
	}
}

func (s *L1SyncerTestSuite) setupClientMock(genesisHash common.Hash, returnBlockByHash bool) {
	s.T().Helper()

	s.rpcClient.GetBlockFunc = func(
		_ context.Context, shardId coreTypes.ShardId, blockId any, fullTx bool,
	) (*types.Block, error) {
		strId, ok := blockId.(string)
		if ok && strId == "earliest" {
			block := testaide.NewGenesisBlock(shardId)
			block.Hash = genesisHash
			return block, nil
		}

		if !returnBlockByHash {
			return nil, nil
		}

		blockHash, ok := blockId.(common.Hash)
		if !ok {
			return nil, fmt.Errorf("unexpected blockId: %v", blockId)
		}
		block := testaide.NewExecutionShardBlock()
		block.ShardId = shardId
		block.Hash = blockHash
		return block, nil
	}
}

func (s *L1SyncerTestSuite) setLatestFinalizedHash(latestFinalizedHash common.Hash) {
	s.T().Helper()
	s.rollupContract.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return latestFinalizedHash, nil
	}
}
