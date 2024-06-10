package main

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cmd/nil_cli/block"
	"github.com/NilFoundation/nil/cmd/nil_cli/contract"
	"github.com/NilFoundation/nil/cmd/nil_cli/keygen"
	"github.com/NilFoundation/nil/cmd/nil_cli/message"
	"github.com/NilFoundation/nil/cmd/nil_cli/receipt"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	RPCEndpoint string `mapstructure:"rpc_endpoint"`
	PrivateKey  string `mapstructure:"private_key"`
}

type RootCommand struct {
	baseCmd *cobra.Command
	config  Config
}

var logger = logging.NewLogger("rootCommand")

func main() {
	rootCommand := &RootCommand{
		baseCmd: &cobra.Command{
			Short: "CLI tool for interacting with the =nil; cluster",
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
		block.GetCommand(rc.config.RPCEndpoint),
		message.GetCommand(rc.config.RPCEndpoint),
		receipt.GetCommand(rc.config.RPCEndpoint),
		contract.GetCommand(rc.config.RPCEndpoint, rc.config.PrivateKey),
	)

	logger.Info().Msg("Subcommands registered")
}

// loadConfig loads the configuration from the config file
func (rc *RootCommand) loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")

	if err := viper.ReadInConfig(); err != nil {
		logger.Fatal().Err(err).Msg("Error reading config file")
	}

	if err := viper.Unmarshal(&rc.config); err != nil {
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

	logger.Info().Msg("Command executed successfully")
}
