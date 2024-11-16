package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/rs/zerolog"
)

type LocalShardApi struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	ShardId  types.ShardId
	msgpool  msgpool.Pool

	nodeApi NodeApi
}

var (
	_ ShardApiRo = (*LocalShardApi)(nil)
	_ ShardApi   = (*LocalShardApi)(nil)
)

func NewLocalShardApi(shardId types.ShardId, db db.ReadOnlyDB, msgpool msgpool.Pool) *LocalShardApi {
	stateAccessor := execution.NewStateAccessor()
	return &LocalShardApi{
		db:       db,
		accessor: stateAccessor,
		ShardId:  shardId,
		msgpool:  msgpool,
	}
}

func (api *LocalShardApi) setAsP2pRequestHandlersIfAllowed(ctx context.Context, networkManager *network.Manager, readonly bool, logger zerolog.Logger) error {
	return SetRawApiRequestHandlers(ctx, api.ShardId, api, networkManager, readonly, logger)
}

func (api *LocalShardApi) setNodeApi(nodeApi NodeApi) {
	api.nodeApi = nodeApi
}
