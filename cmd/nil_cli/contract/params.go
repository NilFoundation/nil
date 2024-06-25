package contract

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	abiFlag      = "abi"
	amountFlag   = "amount"
	noSignFlag   = "no-sign"
	noWaitFlag   = "no-wait"
	saltFlag     = "salt"
	shardIdFlag  = "shard-id"
	gasLimitFlag = "gas-limit"
)

var params = &contractParams{}

type contractParams struct {
	abiPath  string
	noSign   bool
	noWait   bool
	salt     types.Uint256
	shardId  types.ShardId
	gasLimit types.Uint256
}
