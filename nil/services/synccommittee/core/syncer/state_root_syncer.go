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

type L1StateRootGetter interface {
	LatestFinalizedStateRoot(ctx context.Context) (common.Hash, error)
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
	l1StateRootGetter      L1StateRootGetter
	localStateRootAccessor LocalStateRootAccessor
	logger                 logging.Logger
	config                 StateRootSyncerConfig
}

func NewStateRootSyncer(
	fetcher L2BlockFetcher,
	l1StateRootGetter L1StateRootGetter,
	localStateRootAccessor LocalStateRootAccessor,
	logger logging.Logger,
	config StateRootSyncerConfig,
) *stateRootSyncer {
	return &stateRootSyncer{
		fetcher:                fetcher,
		l1StateRootGetter:      l1StateRootGetter,
		localStateRootAccessor: localStateRootAccessor,
		logger:                 logger,
		config:                 config,
	}
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

	latestStateRoot, err := s.l1StateRootGetter.LatestFinalizedStateRoot(ctx)
	if err != nil {
		return nil, err
	}

	if latestStateRoot == common.EmptyHash {
		s.logger.Warn().Msg("Storage state root is not initialized, genesis state root will be used")
		genesisRef, err := s.fetcher.GetGenesisBlockRef(ctx, coreTypes.MainShardId)
		if err != nil {
			return nil, err
		}
		return &genesisRef.Hash, nil
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
