package wallet

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
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
	asJsonFlag       = "json"
	compileInput     = "compile-input"
)

var params = &walletParams{
	Params: &common.Params{},
}

type walletParams struct {
	*common.Params

	deploy          bool
	noWait          bool
	amount          types.Value
	newWalletAmount types.Value
	salt            types.Uint256
	shardId         types.ShardId
	value           types.Value
	currency        types.Value
	currencies      []string
	compileInput    string
}
