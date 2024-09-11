package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Api interface {
	GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.RawBlockWithExtractedData, error)
}
