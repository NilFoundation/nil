package common

import (
	"fmt"

	"github.com/spf13/viper"
)

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
