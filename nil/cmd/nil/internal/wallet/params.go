package wallet

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	abiFlag          = "abi"
	amountFlag       = "amount"
	noWaitFlag       = "no-wait"
	saltFlag         = "salt"
	shardIdFlag      = "shard-id"
	feeCreditFlag    = "fee-credit"
	valueFlag        = "value"
	deployFlag       = "deploy"
	tokenFlag        = "token"
	inOverridesFlag  = "in-overrides"
	outOverridesFlag = "out-overrides"
	withDetailsFlag  = "with-details"
)

var params = &walletParams{}

type walletParams struct {
	abiPath          string
	amount           types.Value
	deploy           bool
	newWalletAmount  types.Value
	noWait           bool
	salt             types.Uint256
	shardId          types.ShardId
	feeCredit        types.Value
	value            types.Value
	currency         types.Value
	currencies       []string
	inOverridesPath  string
	outOverridesPath string
	withDetails      bool
}
