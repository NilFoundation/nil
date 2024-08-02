package message

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	shardIdFlag = "shard-id"
)

var params = &messageParams{}

type messageParams struct {
	shardId types.ShardId
}
