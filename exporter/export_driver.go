package exporter

import (
	"context"

	"github.com/NilFoundation/nil/core/types"
)

type ExportDriver interface {
	SetupScheme(context.Context) error
	ExportBlock(context.Context, types.ShardId, *types.Block) error
	ExportBlocks(context.Context, types.ShardId, []*types.Block) error
	FetchLatestBlock(context.Context, types.ShardId) (*types.Block, error)
	FetchEarlierPoint(context.Context, types.ShardId) (*types.Block, error)
}
