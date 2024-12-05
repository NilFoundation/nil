package config

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logger = logging.NewLogger("configCommand")

var noConfigCmd map[string]struct{} = map[string]struct{}{
	"help": {},
	"init": {},
	"set":  {},
}

var supportedOptions map[string]struct{} = map[string]struct{}{
	"rpc_endpoint":    {},
	"cometa_endpoint": {},
	"faucet_endpoint": {},
	"private_key":     {},
	"address":         {},
}

func GetCommand(configPath *string) *cobra.Command {
	configCmd := &cobra.Command{
		Use:          "config",
		Short:        "Manage the =nil; CLI config",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			SetConfigFile(*configPath)

			if _, withoutConfig := noConfigCmd[cmd.Name()]; withoutConfig {
				return nil
			}

			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read the config file: %w", err)
			}
			return nil
		},
	}

	initCmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize the config file",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := InitDefaultConfig(*configPath)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to create the config file")
				return err
			}

			logger.Info().Msgf("The config file has been initialized successfully: %s", path)
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:          "show",
		Short:        "Show the contents of the config file",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			const printFormat = "%-18s: %v\n"
			fmt.Printf(printFormat, "The config file", viper.ConfigFileUsed())
			nilSection, ok := viper.AllSettings()["nil"].(map[string]interface{})
			if !ok {
				return nil
			}
			for key, value := range nilSection {
				fmt.Printf(printFormat, key, value)
			}
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:          "get [key]",
		Short:        "Get the value of a key from the config file",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := viper.Get("nil." + key)
			if value == nil {
				logger.Warn().Msgf("Key %q is not found in the config file", key)
				return nil
			}
			fmt.Printf("%s: %v\n", key, value)
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:          "set [key] [value]",
		Short:        "Set the value of a key in the config file",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, supported := supportedOptions[args[0]]; !supported {
				logger.Error().Msgf("Key %q is not known", args[0])
				return nil
			}

			if err := PatchConfig(map[string]interface{}{
				args[0]: args[1],
			}, true); err != nil {
				logger.Error().Err(err).Msg("Failed to set the config value")
				return err
			}
			logger.Info().Msgf("Set %q to %q", args[0], args[1])
			return nil
		},
	}

	configCmd.AddCommand(initCmd, showCmd, getCmd, setCmd)

	return configCmd
}
