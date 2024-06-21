package contract

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	abiFlag     = "abi"
	amountFlag  = "amount"
	noSignFlag  = "no-sign"
	saltFlag    = "salt"
	shardIdFlag = "shard-id"
)

var params = &contractParams{}

type contractParams struct {
	abiPath string
	amount  types.Uint256
	noSign  bool
	salt    types.Uint256
	shardId types.ShardId
}
