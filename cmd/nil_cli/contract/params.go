package contract

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	abiFlag     = "abi"
	amountFlag  = "amount"
	saltFlag    = "salt"
	shardIdFlag = "shard-id"
)

var params = &contractParams{}

type contractParams struct {
	salt    types.Uint256
	shardId types.ShardId
	abiPath string
	amount  types.Uint256
}
