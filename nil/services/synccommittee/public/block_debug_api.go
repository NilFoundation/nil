package public

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
)

const (
	DebugBlocksNamespace  = "DebugBlocks"
	DebugGetStateRootData = DebugBlocksNamespace + "_getStateRootData"
	DebugGetLatestFetched = DebugBlocksNamespace + "_getLatestFetched"
	DebugGetBatchView     = DebugBlocksNamespace + "_getBatchView"
	DebugGetBatchViews    = DebugBlocksNamespace + "_getBatchViews"
	DebugGetBatchStats    = DebugBlocksNamespace + "_getBatchStats"
)

type StateRootData struct {
	L1StateRoot    common.Hash  `json:"l1StateRoot"`
	LocalStateRoot *common.Hash `json:"localStateRoot"`
}

func NewStateRootData(
	l1StateRoot common.Hash,
	localStateRoot *common.Hash,
) *StateRootData {
	return &StateRootData{
		L1StateRoot:    l1StateRoot,
		LocalStateRoot: localStateRoot,
	}
}

type BatchDebugRequest struct {
	ListRequest
}

func NewBatchDebugRequest(
	limit *int,
) *BatchDebugRequest {
	return &BatchDebugRequest{
		ListRequest: newListRequestCommon(limit),
	}
}

func DefaultBatchDebugRequest() BatchDebugRequest {
	return *NewBatchDebugRequest(nil)
}

type BatchStats struct {
	TotalCount  int `json:"totalCount"`
	SealedCount int `json:"sealedCount"`
	ProvedCount int `json:"provedCount"`
}

func NewBatchStats(
	totalCount int,
	sealedCount int,
	provedCount int,
) *BatchStats {
	return &BatchStats{
		TotalCount:  totalCount,
		SealedCount: sealedCount,
		ProvedCount: provedCount,
	}
}

type BlockDebugApi interface {
	// GetStateRootData retrieves the current state root data, including the L1 state root and the local state root.
	GetStateRootData(ctx context.Context) (*StateRootData, error)

	// GetLatestFetched retrieves references to the latest fetched blocks for all shards.
	GetLatestFetched(ctx context.Context) (BlockRefs, error)

	// GetBatchView retrieves detailed information about a specific block batch identified by the given BatchId.
	GetBatchView(ctx context.Context, batchId BatchId) (*BatchViewDetailed, error)

	// GetBatchViews retrieves a list of compact batch views based on the given BatchDebugRequest parameters.
	GetBatchViews(ctx context.Context, request BatchDebugRequest) ([]*BatchViewCompact, error)

	// GetBatchStats retrieves statistics about batches currently persisted in the storage.
	GetBatchStats(ctx context.Context) (*BatchStats, error)
}
