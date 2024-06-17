package config

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/core/types"
)

type Config struct {
	RPCEndpoint string            `mapstructure:"rpc_endpoint"`
	PrivateKey  *ecdsa.PrivateKey `mapstructure:"private_key"`
	Address     types.Address     `mapstructure:"address"`
}
