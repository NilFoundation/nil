package client

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type TxPoolClient interface {
	GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (jsonrpc.TxPoolStatus, error)
	GetTxpoolContent(ctx context.Context, shardId types.ShardId) (jsonrpc.TxPoolContent, error)
}
