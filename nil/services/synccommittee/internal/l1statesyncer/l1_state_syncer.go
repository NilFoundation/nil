package l1statesyncer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
)

type l1StateSyncerStorage interface {
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error
}

type l2BlockFetcher interface {
	GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
}

type L1StateSyncer struct {
	storage               l1StateSyncerStorage
	rollupContractWrapper rollupcontract.Wrapper
	l2BlockFetcher        l2BlockFetcher
	logger                logging.Logger
}

// NewL1StateSyncer creates a proposer instance.
func NewL1StateSyncer(
	storage l1StateSyncerStorage,
	contractWrapper rollupcontract.Wrapper,
	l2BlockFetcher l2BlockFetcher,
	logger logging.Logger,
) *L1StateSyncer {
	p := &L1StateSyncer{
		storage:               storage,
		rollupContractWrapper: contractWrapper,
		l2BlockFetcher:        l2BlockFetcher,
		logger:                logger,
	}
	return p
}

func (p *L1StateSyncer) SyncStoredStateRootWithL1(ctx context.Context) error {
	p.logger.Info().Msg("syncing stored state root with L1")

	latestStateRoot, err := p.rollupContractWrapper.LatestFinalizedStateRoot(ctx)
	if err != nil {
		return err
	}

	if latestStateRoot == common.EmptyHash {
		p.logger.Warn().
			Err(err).
			Stringer("latestStateRoot", latestStateRoot).
			Msg("L1 state root is not initialized, genesis state root will be used")

		genesisBlock, err := p.l2BlockFetcher.GetBlock(ctx, types.MainShardId, "earliest", false)
		if err != nil {
			return err
		}
		latestStateRoot = genesisBlock.Hash
	}

	if err := p.storage.SetProvedStateRoot(ctx, latestStateRoot); err != nil {
		return fmt.Errorf("failed set proved state root: %w", err)
	}

	p.logger.Info().
		Stringer("stateRoot", latestStateRoot).
		Msg("stored state root synced")
	return nil
}
