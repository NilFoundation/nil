package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/block"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/config"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/contract"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/keygen"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/message"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/minter"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/receipt"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/system"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/version"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/wallet"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RootCommand struct {
	baseCmd  *cobra.Command
	config   common.Config
	cfgFile  string
	logLevel string
	verbose  bool
}

var logger = logging.NewLogger("root")

var noConfigCmd = map[string]struct{}{
	"help":             {},
	"keygen":           {},
	"completion":       {},
	"__complete":       {},
	"__completeNoDesc": {},
	"config":           {},
	"version":          {},
}

func main() {
	var rootCmd *RootCommand

	rootCmd = &RootCommand{
		baseCmd: &cobra.Command{
			Use:   "nil_cli",
			Short: "CLI tool for interacting with the =nil; cluster",
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				if !rootCmd.verbose {
					zerolog.SetGlobalLevel(zerolog.Disabled)
				} else {
					logLevel, err := zerolog.ParseLevel(rootCmd.logLevel)
					check.PanicIfErr(err)
					zerolog.SetGlobalLevel(logLevel)
				}

				// Set the config file for all commands because some commands can write something to it.
				// E.g. "keygen" command writes a private key to the config file (and creates if it doesn't exist)
				common.SetConfigFile(rootCmd.cfgFile)

				// Traverse up to find the top-level command
				for cmd.HasParent() && cmd.Parent() != rootCmd.baseCmd {
					cmd = cmd.Parent()
				}

				if _, withoutConfig := noConfigCmd[cmd.Name()]; withoutConfig {
					return nil
				}
				if err := rootCmd.loadConfig(); err != nil {
					return err
				}
				if err := rootCmd.validateConfig(); err != nil {
					return err
				}
				common.InitRpcClient(&rootCmd.config, logger)
				return nil
			},
			SilenceUsage:  true,
			SilenceErrors: true,
		},
	}

	rootCmd.baseCmd.PersistentFlags().StringVarP(&rootCmd.cfgFile, "config", "c", common.DefaultConfigPath, "Path to config file")
	rootCmd.baseCmd.PersistentFlags().StringVarP(&rootCmd.logLevel, "log-level", "l", "info", "Log level: trace|debug|info|warn|error|fatal|panic")
	rootCmd.baseCmd.PersistentFlags().BoolVarP(
		&config.Quiet,
		"quiet",
		"q",
		false,
		"Quiet mode (print only the result and exit)",
	)
	rootCmd.baseCmd.PersistentFlags().BoolVarP(
		&rootCmd.verbose,
		"verbose",
		"v",
		false,
		"Verbose mode (print logs)",
	)

	rootCmd.registerSubCommands()
	rootCmd.Execute()
}

// registerSubCommands adds all subcommands to the root command
func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		block.GetCommand(&rc.config),
		config.GetCommand(&rc.cfgFile, &rc.config),
		contract.GetCommand(&rc.config),
		keygen.GetCommand(),
		message.GetCommand(&rc.config),
		minter.GetCommand(&rc.config),
		receipt.GetCommand(&rc.config),
		system.GetCommand(&rc.config),
		version.GetCommand(),
		wallet.GetCommand(&rc.config),
	)
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

// loadConfig loads the configuration from the config file
func (rc *RootCommand) loadConfig() error {
	err := viper.ReadInConfig()

	// Create file if it doesn't exist
	if errors.As(err, new(viper.ConfigFileNotFoundError)) {
		logger.Info().Msg("Config file not found. Creating a new one...")

		path, errCfg := common.InitDefaultConfig(rc.cfgFile)
		if errCfg != nil {
			logger.Error().Err(err).Msg("Failed to create config")
			return err
		}

		logger.Info().Msgf("Config file created successfully at %s", path)
		logger.Info().Msgf("set via `%s config set <option> <value>` or via config file", os.Args[0])
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := viper.UnmarshalKey("nil", &rc.config, updateDecoderConfig); err != nil {
		return fmt.Errorf("unable to decode config: %w", err)
	}

	logger.Debug().Msg("Configuration loaded successfully")
	return nil
}

// validateConfig perform some simple configuration validation
func (rc *RootCommand) validateConfig() error {
	if rc.config.RPCEndpoint == "" {
		logger.Info().Msg("RPCEndpoint is missing in config")
		logger.Info().Msgf("set via `%s config set rpc_endpoint <endpoint>` or via config file", os.Args[0])
		return fmt.Errorf("%q is missing in config", common.RPCEndpointField)
	}
	return nil
}

// Execute runs the root command and handles any errors
func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
