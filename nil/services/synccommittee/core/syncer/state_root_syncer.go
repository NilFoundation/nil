package syncer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type L2BlockFetcher interface {
	TryGetBlockRef(ctx context.Context, shardId coreTypes.ShardId, hash common.Hash) (*types.BlockRef, error)
	GetGenesisBlockRef(ctx context.Context, shardId coreTypes.ShardId) (*types.BlockRef, error)
}

type L1StateRootAccessor interface {
	SetGenesisStateRoot(ctx context.Context, genesisStateRoot common.Hash) error
	GetLatestFinalizedStateRoot(ctx context.Context) (common.Hash, error)
}

type LocalStateRootAccessor interface {
	TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error)
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error
}

type StateRootSyncerConfig struct {
	// AlwaysSyncWithL1 determines if the locally stored state root should always
	// be overridden with the latest finalized state root from L1 during synchronization.
	//
	// If false, the local state root will only be overridden in the following cases:
	//  1. It is empty.
	//  2. Block with the corresponding hash does not exist anymore on L2.
	AlwaysSyncWithL1 bool
}

func NewConfig(alwaysSyncWithL1 bool) StateRootSyncerConfig {
	return StateRootSyncerConfig{
		AlwaysSyncWithL1: alwaysSyncWithL1,
	}
}

func NewDefaultConfig() StateRootSyncerConfig {
	return NewConfig(true)
}

type stateRootSyncer struct {
	fetcher                L2BlockFetcher
	l1StateRootAccessor    L1StateRootAccessor
	localStateRootAccessor LocalStateRootAccessor
	logger                 logging.Logger
	config                 StateRootSyncerConfig
}

func NewStateRootSyncer(
	fetcher L2BlockFetcher,
	l1StateRootGetter L1StateRootAccessor,
	localStateRootAccessor LocalStateRootAccessor,
	logger logging.Logger,
	config StateRootSyncerConfig,
) *stateRootSyncer {
	return &stateRootSyncer{
		fetcher:                fetcher,
		l1StateRootAccessor:    l1StateRootGetter,
		localStateRootAccessor: localStateRootAccessor,
		logger:                 logger,
		config:                 config,
	}
}

// EnsureL1StateIsInitialized ensures the L1 genesis state root is initialized by setting it if it is currently unset.
func (s *stateRootSyncer) EnsureL1StateIsInitialized(ctx context.Context) error {
	s.logger.Info().Msg("Checking if L1 genesis state root is set")

	latestFinalized, err := s.l1StateRootAccessor.GetLatestFinalizedStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get local state root: %w", err)
	}

	if latestFinalized != common.EmptyHash {
		s.logger.Info().Stringer(logging.FieldStateRoot, &latestFinalized).Msg("L1 state root is initialized")
		return nil
	}

	s.logger.Info().Msg("L1 state root is not initialized, genesis block hash will be fetched")

	genesisRef, err := s.fetcher.GetGenesisBlockRef(ctx, coreTypes.MainShardId)
	if err != nil {
		return fmt.Errorf("failed to get genesis block ref: %w", err)
	}

	if err := s.l1StateRootAccessor.SetGenesisStateRoot(ctx, genesisRef.Hash); err != nil {
		return fmt.Errorf("failed to set L1 genesis state root: %w", err)
	}

	s.logger.Info().Stringer(logging.FieldStateRoot, genesisRef.Hash).Msg("L1 genesis state root is set")
	return nil
}

// SyncLatestFinalizedRoot synchronizes the locally stored state root
// with the latest finalized root from L1 or genesis block.
func (s *stateRootSyncer) SyncLatestFinalizedRoot(ctx context.Context) error {
	localRoot, err := s.localStateRootAccessor.TryGetProvedStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get local state root: %w", err)
	}

	if localRoot == nil {
		return s.updateLocalStateRoot(ctx)
	}

	existsOnL2, err := s.existsOnL2(ctx, *localRoot)
	if err != nil {
		return err
	}

	if existsOnL2 && !s.config.AlwaysSyncWithL1 {
		s.logger.Info().
			Stringer(logging.FieldStateRoot, localRoot).
			Msg("Local state root is initialized and in sync with L2, skipping L1 sync (AlwaysSyncWithL1 is false)")
		return nil
	}

	if !existsOnL2 {
		s.logger.Warn().
			Stringer(logging.FieldStateRoot, localRoot).
			Msg("Block with the corresponding hash does not exist on L2, trying to sync")
	}

	return s.updateLocalStateRoot(ctx)
}

func (s *stateRootSyncer) updateLocalStateRoot(ctx context.Context) error {
	latestStateRoot, err := s.getLatestFinalizedRoot(ctx)
	if err != nil {
		return fmt.Errorf(
			"%w: failed to get latest finalized state root: %w", types.ErrStateRootNotSynced, err,
		)
	}

	if err := s.localStateRootAccessor.SetProvedStateRoot(ctx, *latestStateRoot); err != nil {
		return fmt.Errorf(
			"%w: failed to set proved state root: %w", types.ErrStateRootNotSynced, err,
		)
	}

	s.logger.Info().Stringer(logging.FieldStateRoot, latestStateRoot).Msg("Local finalized state root is updated")
	return nil
}

// getLatestFinalizedRoot attempts to retrieve the finalized root from the following sources,
// in order of priority:
// 1. L1 contract
// 2. Genesis block on L2
func (s *stateRootSyncer) getLatestFinalizedRoot(ctx context.Context) (*common.Hash, error) {
	s.logger.Info().Msg("Syncing state with L1")

	latestStateRoot, err := s.l1StateRootAccessor.GetLatestFinalizedStateRoot(ctx)
	if err != nil {
		return nil, err
	}

	if latestStateRoot == common.EmptyHash {
		return nil, types.ErrL1StateRootNotInitialized
	}

	existsOnL2, err := s.existsOnL2(ctx, latestStateRoot)
	if err != nil {
		return nil, err
	}
	if !existsOnL2 {
		return nil, fmt.Errorf("state root %s retrieved from L1 does not exist on L2 side", latestStateRoot)
	}

	return &latestStateRoot, nil
}

func (s *stateRootSyncer) existsOnL2(ctx context.Context, stateRoot common.Hash) (bool, error) {
	ref, err := s.fetcher.TryGetBlockRef(ctx, coreTypes.MainShardId, stateRoot)
	if err != nil {
		return false, fmt.Errorf("failed to get block ref for state root: %w", err)
	}
	if ref == nil {
		return false, nil
	}
	return true, nil
}
