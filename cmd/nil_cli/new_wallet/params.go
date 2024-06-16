package new_wallet

import (
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
)

const (
	shardIdFlag = "shard-id"
	codeFlag    = "code"
	saltFlag    = "salt"
)

var params = &walletParams{}

type walletParams struct {
	shardId types.ShardId
	code    types.Code
	salt    types.Uint256
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *walletParams) initRawParams(cfg *config.Config) error {
	// TODO: Be able to create wallets in any shard.
	p.shardId = types.BaseShardId

	if len(p.code) == 0 {
		p.code = defaultWalletCode(cfg.PrivateKey)
	}
	return nil
}
