package wallet

import (
	"github.com/NilFoundation/nil/core/types"
)

const (
	abiFlag     = "abi"
	amountFlag  = "amount"
	noWaitFlag  = "no-wait"
	saltFlag    = "salt"
	shardIdFlag = "shard-id"
)

var params = &walletParams{}

type walletParams struct {
	abiPath string
	amount  types.Uint256
	noWait  bool
	salt    types.Uint256
	shardId types.ShardId
}
