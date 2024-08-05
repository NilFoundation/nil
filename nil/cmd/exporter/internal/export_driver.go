package internal

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
)

type BlockWithShardId struct {
	*types.BlockWithExtractedData
	ShardId types.ShardId
}

type ExportDriver interface {
	SetupScheme(context.Context) error
	ExportBlocks(context.Context, []*BlockWithShardId) error
	FetchBlock(context.Context, types.ShardId, types.BlockNumber) (*types.Block, bool, error)
	FetchLatestProcessedBlock(context.Context, types.ShardId) (*types.Block, bool, error)
	FetchEarliestAbsentBlock(context.Context, types.ShardId) (types.BlockNumber, bool, error)
	FetchNextPresentBlock(context.Context, types.ShardId, types.BlockNumber) (types.BlockNumber, bool, error)
	Reconnect() error
}
