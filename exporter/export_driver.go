package exporter

import (
	"context"

	"github.com/NilFoundation/nil/core/types"
)

type BlockMsg struct {
	Receipts []*types.Receipt
	Messages []*types.Message
	Block    *types.Block
	Shard    types.ShardId
}

type ExportDriver interface {
	SetupScheme(context.Context) error
	ExportBlocks(context.Context, []*BlockMsg) error
	FetchBlock(context.Context, types.ShardId, types.BlockNumber) (*types.Block, bool, error)
	FetchLatestProcessedBlock(context.Context, types.ShardId) (*types.Block, bool, error)
	FetchEarliestAbsentBlock(context.Context, types.ShardId) (types.BlockNumber, bool, error)
	FetchNextPresentBlock(context.Context, types.ShardId, types.BlockNumber) (types.BlockNumber, bool, error)
}
