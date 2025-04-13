package fetching

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type RpcBlockFetcher interface {
	GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlocksRange(
		ctx context.Context,
		shardId types.ShardId,
		from, to types.BlockNumber,
		fullTx bool,
		batchSize int,
	) ([]*jsonrpc.RPCBlock, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
}
