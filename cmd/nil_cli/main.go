package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/NilFoundation/nil/cmd/nil_cli/block"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/cmd/nil_cli/contract"
	"github.com/NilFoundation/nil/cmd/nil_cli/keygen"
	"github.com/NilFoundation/nil/cmd/nil_cli/message"
	"github.com/NilFoundation/nil/cmd/nil_cli/receipt"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RootCommand struct {
	baseCmd *cobra.Command
	config  *config.Config
}

var logger = logging.NewLogger("rootCommand")

func main() {
	var rootCommand *RootCommand

	rootCommand = &RootCommand{
		baseCmd: &cobra.Command{
			Short: "CLI tool for interacting with the =nil; cluster",
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				if rootCommand.config == nil {
					err := errors.New("Config required")
					logger.Fatal().Err(err).Send()
					return err
				}
				return nil
			},
		},
	}

	rootCommand.loadConfig()
	rootCommand.registerSubCommands()
	rootCommand.Execute()
}

// registerSubCommands adds all subcommands to the root command
func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		keygen.GetCommand(),
		block.GetCommand(rc.config),
		message.GetCommand(rc.config),
		receipt.GetCommand(rc.config),
		contract.GetCommand(rc.config),
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
func (rc *RootCommand) loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")

	if err := viper.ReadInConfig(); err != nil {
		if reflect.TypeOf(err) == reflect.TypeOf(viper.ConfigFileNotFoundError{}) {
			logger.Warn().Msg("No config file found")
			return
		}
		logger.Fatal().Err(err).Msg("Error reading config file")
	}

	if err := viper.Unmarshal(&rc.config, updateDecoderConfig); err != nil {
		logger.Fatal().Err(err).Msg("Unable to decode into config struct")
	}

	logger.Info().Msg("Configuration loaded successfully")
}

// Execute runs the root command and handles any errors
func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}

	logger.Trace().Msg("Command executed successfully")
}
