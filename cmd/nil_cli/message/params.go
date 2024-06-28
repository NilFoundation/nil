package message

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	shardIdFlag = "shard-id"
)

var params = &messageParams{}

type messageParams struct {
	shardId types.ShardId
}
