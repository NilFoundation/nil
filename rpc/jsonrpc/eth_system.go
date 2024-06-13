package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
)

// ChainId implements eth_chainId. Returns the current ethereum chainId.
func (api *APIImpl) ChainId(_ context.Context) (hexutil.Uint64, error) {
	return hexutil.Uint64(types.DefaultChainId), nil
}
