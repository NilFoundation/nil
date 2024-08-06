package wallet

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	abiFlag       = "abi"
	amountFlag    = "amount"
	noWaitFlag    = "no-wait"
	saltFlag      = "salt"
	shardIdFlag   = "shard-id"
	feeCreditFlag = "fee-credit"
	tokenFlag     = "token"
)

var params = &walletParams{}

type walletParams struct {
	abiPath           string
	amount            types.Value
	new_wallet_amount types.Value
	noWait            bool
	salt              types.Uint256
	shardId           types.ShardId
	feeCredit         types.Value
	currency          types.Value
	currencies        []string
}
