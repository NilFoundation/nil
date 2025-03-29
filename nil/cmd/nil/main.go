package main

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/block"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/cometa"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/contract"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/debug"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/keygen"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/minter"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/receipt"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/smartaccount"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/system"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/transaction"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/version"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
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
	"abi":              {},
	"config":           {},
	"help":             {},
	"keygen":           {},
	"completion":       {},
	"__complete":       {},
	"__completeNoDesc": {},
	"transaction":      {},
	"version":          {},
}

func main() {
	var rootCmd *RootCommand

	rootCmd = &RootCommand{
		baseCmd: &cobra.Command{
			Use:   "nil",
			Short: "The CLI tool for interacting with the =nil; cluster",
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
				config.SetConfigFile(rootCmd.cfgFile)

				// Traverse up to find the top-level command
				for cmd.HasParent() && cmd.Parent() != rootCmd.baseCmd {
					cmd = cmd.Parent()
				}

				if _, withoutConfig := noConfigCmd[cmd.Name()]; withoutConfig {
					return nil
				}

				var err error
				cfg, err := config.LoadConfig(rootCmd.cfgFile, logger)
				if err != nil {
					return err
				}
				rootCmd.config = *cfg
				common.InitRpcClient(cfg, logger)
				return nil
			},
			SilenceUsage:  true,
			SilenceErrors: true,
		},
	}

	rootCmd.baseCmd.PersistentFlags().StringVarP(
		&rootCmd.cfgFile,
		"config",
		"c",
		config.DefaultConfigPath,
		"The path to the config file")
	rootCmd.baseCmd.PersistentFlags().StringVarP(
		&rootCmd.logLevel,
		"log-level",
		"l",
		"info",
		"Log level: trace|debug|info|warn|error|fatal|panic")
	rootCmd.baseCmd.PersistentFlags().BoolVarP(
		&common.Quiet,
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
		abi.GetCommand(),
		block.GetCommand(&rc.config),
		config.GetCommand(&rc.cfgFile),
		contract.GetCommand(&rc.config),
		keygen.GetCommand(),
		transaction.GetCommand(&rc.cfgFile),
		minter.GetCommand(&rc.config),
		receipt.GetCommand(&rc.config),
		system.GetCommand(&rc.config),
		version.GetCommand(),
		smartaccount.GetCommand(&rc.config),
		debug.GetCommand(),
		cometa.GetCommand(),
	)
}

// Execute runs the root command and handles any errors
func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
