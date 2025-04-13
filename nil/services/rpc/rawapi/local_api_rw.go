package rawapi

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type localShardApiRw struct {
	roApi   *localShardApiRo
	txnpool txnpool.Pool
}

var _ ShardApiRw = (*localShardApiRw)(nil)

func newLocalShardApiRw(roApi *localShardApiRo, txnpool txnpool.Pool) *localShardApiRw {
	return &localShardApiRw{
		roApi:   roApi,
		txnpool: txnpool,
	}
}

func (api *localShardApiRw) shardId() types.ShardId {
	return api.roApi.shardId()
}

func (api *localShardApiRw) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	return setRawApiRequestHandlers(
		ctx,
		reflect.TypeFor[NetworkTransportProtocolRw](),
		reflect.TypeFor[ShardApiRw](),
		api,
		api.roApi.shardId(),
		apiNameRw,
		networkManager,
		logger)
}

func (api *localShardApiRw) setNodeApi(nodeApi NodeApi) {
	api.roApi.nodeApi = nodeApi
}
