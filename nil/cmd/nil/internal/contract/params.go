package contract

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	abiFlag          = "abi"
	amountFlag       = "amount"
	noSignFlag       = "no-sign"
	noWaitFlag       = "no-wait"
	saltFlag         = "salt"
	shardIdFlag      = "shard-id"
	feeCreditFlag    = "fee-credit"
	valueFlag        = "value"
	internalFlag     = "internal"
	deployFlag       = "deploy"
	inOverridesFlag  = "in-overrides"
	outOverridesFlag = "out-overrides"
	withDetailsFlag  = "with-details"
	asJsonFlag       = "json"
)

var params = &contractParams{
	Params: &common.Params{},
}

type contractParams struct {
	*common.Params

	deploy   bool
	internal bool
	noSign   bool
	noWait   bool
	salt     types.Uint256
	shardId  types.ShardId
	value    types.Value
}
