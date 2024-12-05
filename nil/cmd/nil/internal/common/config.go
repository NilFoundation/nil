package common

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/nil/internal/types"
)

type Config struct {
	RPCEndpoint    string            `mapstructure:"rpc_endpoint"`
	CometaEndpoint string            `mapstructure:"cometa_endpoint"`
	FaucetEndpoint string            `mapstructure:"faucet_endpoint"`
	PrivateKey     *ecdsa.PrivateKey `mapstructure:"private_key"`
	Address        types.Address     `mapstructure:"address"`
}
