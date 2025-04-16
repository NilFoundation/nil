package internal

import (
	"context"
	"reflect"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type localShardApiDev struct {
	shard types.ShardId
}

var _ shardApiDev = (*localShardApiDev)(nil)

func newLocalShardApiDev(shardId types.ShardId) *localShardApiDev {
	return &localShardApiDev{shard: shardId}
}

func (api *localShardApiDev) shardId() types.ShardId {
	return api.shard
}

func (api *localShardApiDev) setNodeApi(_ NodeApi) {}

func (api *localShardApiDev) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	return setRawApiRequestHandlers(
		ctx,
		reflect.TypeFor[NetworkTransportProtocolRo](),
		reflect.TypeFor[shardApiRo](),
		api,
		api.shard,
		apiNameRo,
		networkManager,
		logger)
}

func (api *localShardApiDev) DoPanicOnShard(_ context.Context) (uint64, error) {
	go func() {
		time.Sleep(10 * time.Second)
		panic("RPC request for panic on shard")
	}()
	return 0, nil
}
