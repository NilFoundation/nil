package common

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/viper"
)

type Config struct {
	RPCEndpoint string            `mapstructure:"rpc_endpoint"`
	PrivateKey  *ecdsa.PrivateKey `mapstructure:"private_key"`
	Address     types.Address     `mapstructure:"address"`
}

const (
	WalletField      = "wallet"
	PrivateKeyField  = "private_key"
	RPCEndpointField = "rpc_endpoint"
)

func PatchConfig(delta map[string]any, force bool) error {
	for key, value := range delta {
		oldValue := viper.GetString(key)
		if !force && oldValue != "" && oldValue != value {
			return fmt.Errorf("key %q already exists in the config file", key)
		}
		viper.Set(key, value)
	}

	if err := viper.MergeConfigMap(delta); err != nil {
		return err
	}
	return viper.WriteConfig()
}
