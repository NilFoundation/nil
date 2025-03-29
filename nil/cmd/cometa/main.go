package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Command uint

const (
	CommandRun Command = iota + 1
	CommandCreateConfig
)

type config struct {
	command   Command
	cfgFile   string
	cometaCfg cometa.Config
}

func main() {
	cfg := parseArgs()

	var err error

	switch cfg.command {
	case CommandCreateConfig:
		err = processCreateConfig(cfg)
	case CommandRun:
		err = processRun(cfg)
	}

	if err != nil {
		fmt.Printf("Cometa failed: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func processRun(cfg *config) error {
	client := rpc.NewClientWithDefaultHeaders(
		cfg.cometaCfg.NodeEndpoint,
		logging.Nop(),
		map[string]string{
			"User-Agent": "cometa/" + version.GetGitRevCount(),
		},
	)
	ctx := context.Background()
	service, err := cometa.NewService(ctx, &cfg.cometaCfg, client)
	if err != nil {
		return err
	}
	return service.Run(ctx, &cfg.cometaCfg)
}

func processCreateConfig(cfg *config) error {
	if cfg.cfgFile == "" {
		cfg.cfgFile = "./cometa.yaml"
	}

	data, err := yaml.Marshal(cfg.cometaCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cfg.cfgFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	fmt.Printf("Config file %s has been created\n", cfg.cfgFile)
	return nil
}

func parseArgs() *config {
	cfg := &config{}
	cfg.cometaCfg.ResetToDefault()
	cfg.cfgFile = cobrax.GetConfigNameFromArgs()
	if cfg.command != CommandCreateConfig {
		cfg.cometaCfg.InitFromFile(cfg.cfgFile)
	}

	rootCmd := &cobra.Command{
		Use:           "cometa [global flags] [command]",
		Short:         "cometa contracts metadata app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cobrax.AddConfigFlag(rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().BoolVar(&cfg.cometaCfg.UseBadger, "use-badger", cfg.cometaCfg.UseBadger, "use badger db")
	rootCmd.PersistentFlags().StringVar(
		&cfg.cometaCfg.OwnEndpoint, "own-endpoint", cfg.cometaCfg.OwnEndpoint, "cometa's rpc server endpoint")
	rootCmd.PersistentFlags().StringVar(
		&cfg.cometaCfg.NodeEndpoint, "node-endpoint", cfg.cometaCfg.NodeEndpoint, "nil node endpoint")
	rootCmd.PersistentFlags().StringVar(
		&cfg.cometaCfg.DbEndpoint, "db-endpoint", cfg.cometaCfg.DbEndpoint, "database endpoint")
	rootCmd.PersistentFlags().StringVar(
		&cfg.cometaCfg.DbPath, "db-path", cfg.cometaCfg.DbPath, "path where to store database")
	rootCmd.PersistentFlags().StringVar(&cfg.cometaCfg.DbName, "db-name", cfg.cometaCfg.DbName, "database name")
	rootCmd.PersistentFlags().StringVar(&cfg.cometaCfg.DbUser, "db-user", cfg.cometaCfg.DbUser, "database user")
	rootCmd.PersistentFlags().StringVar(
		&cfg.cometaCfg.DbPassword, "db-password", cfg.cometaCfg.DbPassword, "database password")

	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		fmt.Printf("failed to bind flags: %s\n", err.Error())
		os.Exit(1)
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run cometa server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.command = CommandRun
		},
	}

	createConfigCmd := &cobra.Command{
		Use:   "create-config",
		Short: "Create config file",
		Long:  "Create config file which can be specified by `--config` flag. By default it creates `./cometa.yaml`",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.command = CommandCreateConfig
		},
	}
	rootCmd.AddCommand(createConfigCmd, runCmd)

	var logLevel string
	cobrax.AddLogLevelFlag(rootCmd.PersistentFlags(), &logLevel)

	check.PanicIfErr(rootCmd.Execute())

	logging.SetupGlobalLogger(logLevel)

	return cfg
}
