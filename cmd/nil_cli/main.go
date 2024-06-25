package main

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"reflect"

	"github.com/NilFoundation/nil/cmd/nil_cli/block"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/cmd/nil_cli/contract"
	"github.com/NilFoundation/nil/cmd/nil_cli/keygen"
	"github.com/NilFoundation/nil/cmd/nil_cli/message"
	"github.com/NilFoundation/nil/cmd/nil_cli/receipt"
	"github.com/NilFoundation/nil/cmd/nil_cli/wallet"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RootCommand struct {
	baseCmd *cobra.Command
	config  config.Config
	cfgFile string
}

var logger = logging.NewLogger("rootCommand")

var noConfigCmd map[string]struct{} = map[string]struct{}{
	"help":   {},
	"keygen": {},
}

func main() {
	var rootCmd *RootCommand

	rootCmd = &RootCommand{
		baseCmd: &cobra.Command{
			Short: "CLI tool for interacting with the =nil; cluster",
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				if _, withoutConfig := noConfigCmd[cmd.Name()]; withoutConfig {
					return nil
				}
				if err := rootCmd.loadConfig(); err != nil {
					return err
				}
				return nil
			},
		},
	}

	rootCmd.baseCmd.PersistentFlags().StringVarP(&rootCmd.cfgFile, "config", "c", "config.yaml", "Path to config file")

	rootCmd.registerSubCommands()
	rootCmd.Execute()
}

// registerSubCommands adds all subcommands to the root command
func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		keygen.GetCommand(),
		block.GetCommand(&rc.config),
		message.GetCommand(&rc.config),
		receipt.GetCommand(&rc.config),
		contract.GetCommand(&rc.config),
		wallet.GetCommand(&rc.config),
	)

	logger.Trace().Msg("Subcommands registered")
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
	viper.SetConfigFile(rc.cfgFile)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := viper.Unmarshal(&rc.config, updateDecoderConfig); err != nil {
		return fmt.Errorf("unable to decode config: %w", err)
	}

	logger.Info().Msg("Configuration loaded successfully")
	return nil
}

// Execute runs the root command and handles any errors
func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}

	logger.Trace().Msg("Command executed successfully")
}
