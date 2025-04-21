package internal

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type localShardApiRo struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	shard    types.ShardId

	bootstrapConfig *rpctypes.BootstrapConfig

	nodeApi NodeApi
	logger  logging.Logger
}

var _ shardApiRo = (*localShardApiRo)(nil)

func newLocalShardApiRo(
	shardId types.ShardId,
	db db.ReadOnlyDB,
	bootstrapConfig *rpctypes.BootstrapConfig,
) *localShardApiRo {
	stateAccessor := execution.NewStateAccessor()
	return &localShardApiRo{
		bootstrapConfig: bootstrapConfig,
		db:              db,
		accessor:        stateAccessor,
		shard:           shardId,
		logger:          logging.NewLogger("local_api"),
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
		reflect.TypeFor[shardApiRo](),
		api,
		api.shard,
		apiNameRo,
		networkManager,
		logger)
}

func (api *localShardApiRo) setNodeApi(nodeApi NodeApi) {
	api.nodeApi = nodeApi
}
