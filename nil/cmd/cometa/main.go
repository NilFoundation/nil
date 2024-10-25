package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Command uint

const (
	CommandRun Command = iota
	CommandCreateConfig
)

type config struct {
	command   Command
	cfgFile   string
	cometaCfg cometa.Config
}

func main() {
	cfg := parseArgs()

	initConfig(cfg)

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
		zerolog.Nop(),
		map[string]string{
			"User-Agent": "cometa/" + version.GetGitRevision(),
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

	cfgTemplate := fmt.Sprintf(`
own-endpoint: %s
node-endpoint: %s
db-endpoint: %s
db-path: %s
db-name: %s
db-user: %s
db-password: %s
`, cometa.OwnEndpointDefault, cometa.NodeEndpointDefault, cometa.DbEndpointDefault, cometa.DbPathDefault,
		cometa.DbNameDefault, cometa.DbUserDefault, cometa.DbPasswordDefault)

	if err := os.WriteFile(cfg.cfgFile, []byte(cfgTemplate), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	fmt.Printf("Config file %s has been created\n", cfg.cfgFile)
	return nil
}

func initConfig(cfg *config) {
	if cfg.command == CommandCreateConfig {
		return
	}
	if cfg.cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfg.cfgFile)
	} else {
		// Search config in the current directory
		viper.AddConfigPath("./")
		viper.SetConfigName("cometa")
	}
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Run without config file: %s\n", err.Error())
		cfg.cometaCfg.ResetDefualt()
		return
	}
	cfg.cometaCfg.OwnEndpoint = viper.GetString("own-endpoint")
	cfg.cometaCfg.NodeEndpoint = viper.GetString("node-endpoint")
	cfg.cometaCfg.DbEndpoint = viper.GetString("db-endpoint")
	cfg.cometaCfg.DbPath = viper.GetString("db-path")
	cfg.cometaCfg.DbName = viper.GetString("db-name")
	cfg.cometaCfg.DbUser = viper.GetString("db-user")
	cfg.cometaCfg.DbPassword = viper.GetString("db-password")
}

func parseArgs() *config {
	cfg := &config{}
	rootCmd := &cobra.Command{
		Use:           "cometa [global flags] [command]",
		Short:         "cometa contracts metadata app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVarP(&cfg.cfgFile, "config", "c", "", "config file")
	rootCmd.PersistentFlags().BoolVar(&cfg.cometaCfg.UseBadger, "use-badger", false, "use badger db")
	rootCmd.Flags().String("own-endpoint", cometa.OwnEndpointDefault, "cometa's rpc server endpoint")
	rootCmd.Flags().String("node-endpoint", cometa.NodeEndpointDefault, "nil node endpoint")
	rootCmd.Flags().String("db-endpoint", cometa.DbEndpointDefault, "database endpoint")
	rootCmd.Flags().String("db-path", cometa.DbPathDefault, "path where to store database")
	rootCmd.Flags().String("db-name", cometa.DbNameDefault, "database name")
	rootCmd.Flags().String("db-user", cometa.DbUserDefault, "database user")
	rootCmd.Flags().String("db-password", cometa.DbPasswordDefault, "database password")

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
	rootCmd.AddCommand(runCmd)

	createConfigCmd := &cobra.Command{
		Use:   "create-config",
		Short: "Create config file",
		Long:  "Create config file which can be specified by `--config` flag. By default it creates `./cometa.yaml`",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.command = CommandCreateConfig
		},
	}
	rootCmd.AddCommand(createConfigCmd)

	logLevel := rootCmd.PersistentFlags().StringP("log-level", "l", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	logging.SetupGlobalLogger(*logLevel)

	check.PanicIfErr(rootCmd.Execute())

	return cfg
}
