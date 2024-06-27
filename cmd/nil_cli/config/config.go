package config

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var logger = logging.NewLogger("configCommand")

var noConfigCmd map[string]struct{} = map[string]struct{}{
	"help": {},
	"init": {},
}

var supportedOptions map[string]struct{} = map[string]struct{}{
	"rpc_endpoint": {},
	"private_key":  {},
	"address":      {},
}

const intitConfigTemplate = `---
# Configuration for interacting with the =nil; cluster

# Specify the RPC endpoint of your cluster
# For example, if your cluster's RPC endpoint is at "http://127.0.0.1:8529", set it as below
# rpc_endpoint: "http://127.0.0.1:8529"

# Specify the private key used for signing transactions
# This should be a hexadecimal string corresponding to your account's private key
# private_key: "WRITE_YOUR_PRIVATE_KEY_HERE"

# Specify the the address of your wallet to be receipt of your external messages
# This should be a hexadecimal string corresponding to your account's address
# address: "0xWRITE_YOUR_ADDRESS_HERE"
`

func GetCommand(configPath *string, cfg *common.Config) *cobra.Command {
	configCmd := &cobra.Command{
		Use:          "config",
		Short:        "Configuration management",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetConfigFile(*configPath)

			if _, withoutConfig := noConfigCmd[cmd.Name()]; withoutConfig {
				return nil
			}

			if err := viper.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}
			return nil
		},
	}

	initCmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize config file",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.OpenFile(viper.ConfigFileUsed(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to create config file")
				return err
			}
			defer file.Close()

			_, err = file.WriteString(intitConfigTemplate)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to write template to config file")
				return err
			}

			logger.Info().Msgf("Config initialized successfully: %s", viper.ConfigFileUsed())
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:          "show",
		Short:        "Show the config file content",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info().Msgf("Config file: %s", viper.ConfigFileUsed())
			for key, value := range viper.AllSettings() {
				logger.Info().Msgf("%s: %v", key, value)
			}
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:          "get [key]",
		Short:        "Get a config value",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := viper.Get(key)
			if value == nil {
				logger.Warn().Msgf("Key %q is not found in config", key)
				return nil
			}
			logger.Info().Msgf("%s: %v", key, value)
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:          "set [key] [value]",
		Short:        "Set a config value",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, supported := supportedOptions[args[0]]; !supported {
				logger.Error().Msgf("Key %q is not known", args[0])
				return nil
			}

			if err := common.PatchConfig(map[string]interface{}{
				args[0]: args[1],
			}, true); err != nil {
				logger.Error().Err(err).Msg("Failed to set config value")
				return err
			}
			logger.Info().Msgf("Set %q to %q", args[0], args[1])
			return nil
		},
	}

	configCmd.AddCommand(initCmd)
	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(getCmd)
	configCmd.AddCommand(setCmd)

	return configCmd
}
