package smartaccount

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/common"
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
	priorityFee      = "priority-fee"
)

type smartAccountParams struct {
	*common.Params

	deploy                bool
	noWait                bool
	amount                types.Value
	newSmartAccountAmount types.Value
	salt                  types.Uint256
	shardId               types.ShardId
	value                 types.Value
	token                 types.Value
	tokens                []string
	compileInput          string
	priorityFee           string
}
