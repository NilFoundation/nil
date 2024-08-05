package common

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
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
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		// impossible, since we set the default in SetConfigFile
		panic("config file is not set")
	}
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			configPath, err = InitDefaultConfig(configPath)
		}
		if err != nil {
			return err
		}
	}

	cfg, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	result := strings.Builder{}
	first := true
	for _, line := range strings.Split(string(cfg), "\n") {
		if !first {
			result.WriteByte('\n')
		} else {
			first = false
		}
		key := strings.TrimSpace(strings.Split(line, "=")[0])
		if value, ok := delta[key]; ok {
			result.WriteString(fmt.Sprintf("%s = %v", key, value))
			delete(delta, key)
		} else {
			result.WriteString(line)
		}
	}
	for key, value := range delta {
		result.WriteString(fmt.Sprintf("%s = %v\n", key, value))
	}
	return os.WriteFile(configPath, []byte(result.String()), 0o600)
}

// SetConfigFile sets the config file for the viper
func SetConfigFile(cfgFile string) {
	viper.SetConfigType("ini")
	viper.SetConfigFile(cfgFile)
}
