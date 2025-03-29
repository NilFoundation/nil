package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-viper/encoding/ini"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

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

; Specify the RPC endpoint of your Cometa service
; Cometa service is not mandatory, you can leave it empty if you don't use it
; For example, if your Cometa's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; cometa_endpoint = "http://127.0.0.1:8529"

; Specify the RPC endpoint of a Faucet service
; Faucet service is not mandatory, you can leave it empty if you don't use it
; For example, if your Faucet's RPC endpoint is at "http://127.0.0.1:8529", set it as below
; faucet_endpoint = "http://127.0.0.1:8529"

; Specify the private key used for signing external transactions to your smart account.
; You can generate a new key with "nil keygen new".
; private_key = "WRITE_YOUR_PRIVATE_KEY_HERE"

; Specify the address of your smart account to be the receiver of your external transactions.
; You can deploy a new account and save its address with "nil smart account new".
; address = "0xWRITE_YOUR_ADDRESS_HERE"
`

var DefaultConfigPath string

func init() {
	homeDir, err := os.UserHomeDir()
	check.PanicIfErr(err)

	DefaultConfigPath = filepath.Join(homeDir, ".config/nil/config.ini")

	codecRegistry := viper.NewCodecRegistry()
	check.PanicIfErr(codecRegistry.RegisterCodec("ini", ini.Codec{}))
	viper.SetOptions(viper.WithCodecRegistry(codecRegistry))
}

func InitDefaultConfig(configPath string) (string, error) {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	dirPath := filepath.Dir(configPath)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
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

func decodePrivateKey(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() == reflect.String && t == reflect.TypeOf(&ecdsa.PrivateKey{}) {
		s, _ := data.(string)
		return crypto.HexToECDSA(s)
	}
	return data, nil
}

func decodeAddress(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() == reflect.String && t == reflect.TypeOf(types.Address{}) {
		s, _ := data.(string)
		var res types.Address
		if err := res.UnmarshalText([]byte(s)); err != nil {
			return nil, err
		}
		return res, nil
	}
	return data, nil
}

func updateDecoderConfig(config *mapstructure.DecoderConfig) {
	config.DecodeHook = mapstructure.ComposeDecodeHookFunc(
		config.DecodeHook,
		decodePrivateKey,
		decodeAddress,
	)
}

// LoadConfig loads the configuration from the config file
func LoadConfig(cfgFilePath string, logger logging.Logger) (*common.Config, error) {
	err := viper.ReadInConfig()

	// Create file if it doesn't exist
	if errors.As(err, new(viper.ConfigFileNotFoundError)) {
		logger.Info().Msg("Config file not found. Creating a new one...")

		path, errCfg := InitDefaultConfig(cfgFilePath)
		if errCfg != nil {
			logger.Error().Err(err).Msg("Failed to create config")
			return nil, err
		}

		logger.Info().Msgf("Config file created successfully at %s", path)
		logger.Info().Msgf("set via `%s config set <option> <value>` or via config file", os.Args[0])
		return &common.Config{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := common.Config{}
	if err := viper.UnmarshalKey("nil", &config, updateDecoderConfig); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	if err := validateConfig(&config, logger); err != nil {
		return nil, err
	}

	logger.Debug().Msg("Configuration loaded successfully")
	return &config, nil
}

// validateConfig perform some simple configuration validation
func validateConfig(config *common.Config, logger logging.Logger) error {
	if config.RPCEndpoint == "" {
		return MissingKeyError(RPCEndpointField, logger)
	}
	return nil
}

var generateCommands = map[string]string{
	PrivateKeyField: "keygen",
	AddressField:    "smart-account new",
}

func MissingKeyError(key string, logger logging.Logger) error {
	logger.Info().Msgf(
		"%s not specified in config.\nRun `%s config set %s <value>` or set via config file.", key, os.Args[0], key)

	if cmd, ok := generateCommands[key]; ok {
		logger.Info().Msgf("You can also run `%s %s` to generate a new one.", os.Args[0], cmd)
	}

	return fmt.Errorf("%s not specified in config", key)
}
