package rawapi

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type shardApiClientDev struct {
	shardApiRequestPerformer
}

var (
	_ shardApiRequestPerformerSetter = (*shardApiClientDev)(nil)
	_ ShardApiDev                    = (*shardApiClientDev)(nil)
)

func newShardApiClientNetworkDev(shardId types.ShardId, networkManager network.Manager) *shardApiClientDev {
	client, err := newShardApiClientNetwork[*shardApiClientDev](
		shardId,
		apiNameDev,
		networkManager,
		reflect.TypeFor[ShardApiDev](),
		reflect.TypeFor[NetworkTransportProtocolDev]())
	check.PanicIfErr(err)
	return client
}

func (api *shardApiClientDev) setShardApiRequestPerformer(requestPerformer shardApiRequestPerformer) {
	api.shardApiRequestPerformer = requestPerformer
}

func (api *shardApiClientDev) DoPanicOnShard(ctx context.Context) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](
		ctx, api.shardApiRequestPerformer, "DoPanicOnShard")
}
