package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// ChainId implements eth_chainId. Returns the current ethereum chainId.
func (api *APIImplRo) ChainId(_ context.Context) (hexutil.Uint64, error) {
	return hexutil.Uint64(types.DefaultChainId), nil
}

// GasPrice implements Eth_gasPrice. Returns the current gas price in the network for a given shard.
func (api *APIImplRo) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	return api.rawapi.GasPrice(ctx, shardId)
}
