package config

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Quiet = false

var logger = logging.NewLogger("configCommand")

var noConfigCmd map[string]struct{} = map[string]struct{}{
	"help": {},
	"init": {},
	"set":  {},
}

var supportedOptions map[string]struct{} = map[string]struct{}{
	"rpc_endpoint":    {},
	"cometa_endpoint": {},
	"private_key":     {},
	"address":         {},
}

func GetCommand(configPath *string, cfg *common.Config) *cobra.Command {
	configCmd := &cobra.Command{
		Use:          "config",
		Short:        "Configuration management",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			common.SetConfigFile(*configPath)

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
			path, err := common.InitDefaultConfig(*configPath)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to create config")
				return err
			}

			logger.Info().Msgf("Config initialized successfully: %s", path)
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
			nilSection, ok := viper.AllSettings()["nil"].(map[string]interface{})
			if !ok {
				return nil
			}
			for key, value := range nilSection {
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
			value := viper.Get("nil." + key)
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
