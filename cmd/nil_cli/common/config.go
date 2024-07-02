package common

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/viper"
)

type Config struct {
	RPCEndpoint string            `mapstructure:"rpc_endpoint"`
	PrivateKey  *ecdsa.PrivateKey `mapstructure:"private_key"`
	Address     types.Address     `mapstructure:"address"`
}

const (
	AddressField     = "address"
	PrivateKeyField  = "private_key"
	RPCEndpointField = "rpc_endpoint"
)

const InitConfigTemplate = `; Configuration for interacting with the =nil; cluster
[nil]

; Specify the RPC endpoint of your cluster
; For example, if your cluster's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; rpc_endpoint = "http://127.0.0.1:8529"

; Specify the private key used for signing external messages to your wallet.
; You can generate a new key with "nil_cli keygen new".
; private_key = "WRITE_YOUR_PRIVATE_KEY_HERE"

; Specify the address of your wallet to be the receiver of your external messages.
; You can deploy a new wallet and save its address with "nil_cli wallet new".
; address = "0xWRITE_YOUR_ADDRESS_HERE"
`

var DefaultConfigPath string

func init() {
	homeDir, err := os.UserHomeDir()
	check.PanicIfErr(err)

	DefaultConfigPath = filepath.Join(homeDir, ".config/nil/config.ini")
}

func InitDefaultConfig(configPath string) (string, error) {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	dirPath := filepath.Dir(configPath)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directrory: %w", err)
	}

	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(InitConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to write template to config file: %w", err)
	}
	return configPath, nil
}

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

// SetConfigFile sets the config file for the viper
func SetConfigFile(cfgFile string) {
	if cfgFile == "" {
		viper.SetConfigName("config")
		viper.SetConfigType("ini")
		viper.AddConfigPath("$HOME/.config/nil/")
		viper.AddConfigPath(".")
	} else {
		viper.SetConfigFile(cfgFile)
	}
}
