package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
)

type Api interface {
	GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.Block, error)
	GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.BlockWithRawExtractedData, error)
}
