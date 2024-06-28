package receipt

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	shardIdFlag = "shard-id"
)

var params = &receiptParams{}

type receiptParams struct {
	shardId types.ShardId
}
