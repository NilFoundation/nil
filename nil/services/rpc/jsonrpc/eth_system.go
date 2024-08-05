package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// ChainId implements eth_chainId. Returns the current ethereum chainId.
func (api *APIImpl) ChainId(_ context.Context) (hexutil.Uint64, error) {
	return hexutil.Uint64(types.DefaultChainId), nil
}

// GasPrice implements Eth_gasPrice. Returns the current gas price in the network for a given shard.
func (api *APIImpl) GasPrice(ctx context.Context, shardId types.ShardId) (*hexutil.Big, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()
	gasPrice, err := db.ReadGasPerShard(tx, shardId)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return (*hexutil.Big)(big.NewInt(0)), nil
		}
		return nil, err
	}
	return (*hexutil.Big)(gasPrice.ToBig()), nil
}
