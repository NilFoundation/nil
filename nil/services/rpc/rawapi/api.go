package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

type Api interface {
	GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error)
	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
}

type ShardApi interface {
	GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error)
	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
}
