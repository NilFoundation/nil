package rawapi

import (
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type LocalShardApi struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	ShardId  types.ShardId

	nodeApi NodeApi
}

var _ ShardApi = (*LocalShardApi)(nil)

func NewLocalShardApi(shardId types.ShardId, db db.ReadOnlyDB) (*LocalShardApi, error) {
	stateAccessor, err := execution.NewStateAccessor()
	if err != nil {
		return nil, err
	}
	return &LocalShardApi{
		db:       db,
		accessor: stateAccessor,
		ShardId:  shardId,
	}, nil
}

func (api *LocalShardApi) setNodeApi(nodeApi NodeApi) {
	api.nodeApi = nodeApi
}
