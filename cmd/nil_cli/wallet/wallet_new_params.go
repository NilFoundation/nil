package wallet

import (
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	shardIdFlag = "shard-id"
	codeFlag    = "code"
	saltFlag    = "salt"
)

var params = &walletNewParams{}

type walletNewParams struct {
	shardId types.ShardId
	code    types.Code
	salt    types.Uint256
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *walletNewParams) initRawParams(cfg *config.Config) error {
	if len(p.code) == 0 {
		p.code = contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&cfg.PrivateKey.PublicKey))
	}
	return nil
}
