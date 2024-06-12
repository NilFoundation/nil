package main

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cmd/nil_cli/block"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/cmd/nil_cli/contract"
	"github.com/NilFoundation/nil/cmd/nil_cli/keygen"
	"github.com/NilFoundation/nil/cmd/nil_cli/message"
	"github.com/NilFoundation/nil/cmd/nil_cli/receipt"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RootCommand struct {
	baseCmd *cobra.Command
	config  config.Config
}

var logger = logging.NewLogger("rootCommand")

func main() {
	var rootCommand *RootCommand

	rootCommand = &RootCommand{
		baseCmd: &cobra.Command{
			Short: "CLI tool for interacting with the =nil; cluster",
			PersistentPreRun: func(cmd *cobra.Command, args []string) {
				if cmd.CalledAs() != "help" {
					rootCommand.loadConfig()
				}
			},
		},
	}

	rootCommand.registerSubCommands()
	rootCommand.Execute()
}

// registerSubCommands adds all subcommands to the root command
func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		keygen.GetCommand(),
		block.GetCommand(&rc.config),
		message.GetCommand(&rc.config),
		receipt.GetCommand(&rc.config),
		contract.GetCommand(&rc.config),
	)

	logger.Trace().Msg("Subcommands registered")
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

	logger.Trace().Msg("Command executed successfully")
}
