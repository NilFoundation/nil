package rawapi

import (
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
)

type LocalShardApi struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	ShardId  types.ShardId
	msgpool  msgpool.Pool

	nodeApi NodeApi
}

var _ ShardApi = (*LocalShardApi)(nil)

func NewLocalShardApi(shardId types.ShardId, db db.ReadOnlyDB, msgpool msgpool.Pool) *LocalShardApi {
	stateAccessor := execution.NewStateAccessor()
	return &LocalShardApi{
		db:       db,
		accessor: stateAccessor,
		ShardId:  shardId,
		msgpool:  msgpool,
	}
}

func (api *LocalShardApi) setNodeApi(nodeApi NodeApi) {
	api.nodeApi = nodeApi
}
