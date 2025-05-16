package debug

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type DebuggerL1Client interface {
	GetLatestFinalizedStateRoot(ctx context.Context) (common.Hash, error)
}

type DebuggerBlockStorage interface {
	TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error)
	GetLatestFetched(ctx context.Context) (public.BlockRefs, error)
	GetBatchView(ctx context.Context, batchId public.BatchId) (*public.BatchViewDetailed, error)
	GetBatchViews(ctx context.Context, request public.BatchDebugRequest) ([]*public.BatchViewCompact, error)
	GetBatchStats(ctx context.Context) (*public.BatchStats, error)
}

type blockDebugger struct {
	l1Client DebuggerL1Client
	storage  DebuggerBlockStorage
}

func NewBlockDebugger(
	l1Client DebuggerL1Client,
	storage DebuggerBlockStorage,
) public.BlockDebugApi {
	return &blockDebugger{
		l1Client: l1Client,
		storage:  storage,
	}
}

func (d *blockDebugger) GetStateRootData(ctx context.Context) (*public.StateRootData, error) {
	l1StateRoot, err := d.l1Client.GetLatestFinalizedStateRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest finalized state root from L1: %w", err)
	}

	localStateRoot, err := d.storage.TryGetProvedStateRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get locally stored state root: %w", err)
	}

	stateRootData := public.NewStateRootData(l1StateRoot, localStateRoot)
	return stateRootData, nil
}

func (d *blockDebugger) GetLatestFetched(ctx context.Context) (public.BlockRefs, error) {
	return d.storage.GetLatestFetched(ctx)
}

func (d *blockDebugger) GetBatchView(ctx context.Context, batchId public.BatchId) (*public.BatchViewDetailed, error) {
	return d.storage.GetBatchView(ctx, batchId)
}

func (d *blockDebugger) GetBatchViews(
	ctx context.Context, request public.BatchDebugRequest,
) ([]*public.BatchViewCompact, error) {
	return d.storage.GetBatchViews(ctx, request)
}

func (d *blockDebugger) GetBatchStats(ctx context.Context) (*public.BatchStats, error) {
	return d.storage.GetBatchStats(ctx)
}
