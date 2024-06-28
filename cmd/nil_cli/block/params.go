package block

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	shardIdFlag = "shard-id"
)

var params = &blockParams{}

type blockParams struct {
	shardId types.ShardId
}
