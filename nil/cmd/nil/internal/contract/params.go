package contract

import (
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
	inOverridesFlag  = "in-overrides"
	outOverridesFlag = "out-overrides"
	withDetailsFlag  = "with-details"
)

var params = &contractParams{}

type contractParams struct {
	abiPath          string
	noSign           bool
	noWait           bool
	withDetails      bool
	salt             types.Uint256
	shardId          types.ShardId
	feeCredit        types.Value
	inOverridesPath  string
	outOverridesPath string
}
