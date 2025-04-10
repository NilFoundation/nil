package block

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	jsonFlag    = "json"
	fullFlag    = "full"
	noColorFlag = "no-color"
	shardIdFlag = "shard-id"
)

type blockParams struct {
	jsonOutput bool
	fullOutput bool
	noColor    bool
	shardId    types.ShardId
}
