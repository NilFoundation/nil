package rawapi

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type localShardApiRo struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	shard    types.ShardId

	nodeApi NodeApi
	logger  logging.Logger
}

var _ ShardApiRo = (*localShardApiRo)(nil)

func newLocalShardApiRo(shardId types.ShardId, db db.ReadOnlyDB) *localShardApiRo {
	stateAccessor := execution.NewStateAccessor()
	return &localShardApiRo{
		db:       db,
		accessor: stateAccessor,
		shard:    shardId,
		logger:   logging.NewLogger("local_api"),
	}
}

func (api *localShardApiRo) shardId() types.ShardId {
	return api.shard
}

func (api *localShardApiRo) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	return setRawApiRequestHandlers(
		ctx,
		reflect.TypeFor[NetworkTransportProtocolRo](),
		reflect.TypeFor[ShardApiRo](),
		api,
		api.shard,
		apiNameRo,
		networkManager,
		logger)
}

func (api *localShardApiRo) setNodeApi(nodeApi NodeApi) {
	api.nodeApi = nodeApi
}
