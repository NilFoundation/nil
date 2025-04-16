package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/NilFoundation/nil/nil/internal/cobrax/cmdflags"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	reportPeriod = 5 * time.Second

	filePermissions = 0o644

	defaultConfigFileName = "nil-relay.yaml"
)

var logger = logging.NewLogger("relay")

type config struct {
	LogLevel  string `yaml:"logLevel,omitempty"`
	PprofPort int    `yaml:"pprofPort,omitempty"`

	Network   *network.Config   `yaml:"network,omitempty"`
	Telemetry *telemetry.Config `yaml:"telemetry,omitempty"`
}

func runCommand() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg := &config{
		Network:   network.NewDefaultConfig(),
		Telemetry: telemetry.NewDefaultConfig(),
	}
	cfg.Network.ServeRelay = true

	cfgFileName := cobrax.GetConfigNameFromArgs()
	if err := cobrax.LoadConfigFromFile(cfgFileName, cfg); err != nil {
		return err
	}

	rootCmd := &cobra.Command{
		Use:           "nil-relay [flags]",
		Short:         "relay for nild cluster network",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cobrax.AddConfigFlag(rootCmd.PersistentFlags())
	cobrax.AddLogLevelFlag(rootCmd.PersistentFlags(), &cfg.LogLevel)
	cobrax.AddPprofPortFlag(rootCmd.PersistentFlags(), &cfg.PprofPort)
	cmdflags.AddNetwork(rootCmd.PersistentFlags(), cfg.Network)
	cmdflags.AddTelemetry(rootCmd.PersistentFlags(), cfg.Telemetry)

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run libp2p relay server",
		RunE: func(*cobra.Command, []string) error {
			return run(ctx, cfg)
		},
	}
	rootCmd.AddCommand(runCmd)

	outputFile := new(string)
	genConfigCmd := &cobra.Command{
		Use:   "gen-config",
		Short: "Generate default config",
		RunE: func(*cobra.Command, []string) error {
			return genConfig(cfg, *outputFile)
		},
	}
	genConfigCmd.Flags().StringVarP(outputFile, "output", "o", "", "Output config file name")
	rootCmd.AddCommand(genConfigCmd)

	rootCmd.AddCommand(cobrax.VersionCmd("nil-relay"))

	return rootCmd.Execute()
}

func genConfig(cfg *config, fileName string) error {
	if cfg.Network == nil || !cfg.Network.Enabled() {
		return errors.New("network config is disabled; provide a port at least")
	}

	if fileName == "" {
		fileName = defaultConfigFileName
	}

	if _, err := network.LoadOrGenerateKeys(cfg.Network.KeysPath); err != nil {
		return fmt.Errorf("failed to ensure keys file %s existence: %w", cfg.Network.KeysPath, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(fileName, data, filePermissions); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", fileName, err)
	}

	return nil
}

func run(ctx context.Context, cfg *config) error {
	logging.SetupGlobalLogger(cfg.LogLevel)
	profiling.Start(cfg.PprofPort)

	if err := telemetry.Init(ctx, cfg.Telemetry); err != nil {
		return err
	}
	defer telemetry.Shutdown(ctx)

	nm, err := network.NewManager(ctx, cfg.Network, nil)
	if err != nil {
		return err
	}
	defer nm.Close()

	concurrent.RunTickerLoop(ctx, reportPeriod, func(context.Context) {
		logger.Info().Msg("I am still alive")
	})

	logger.Info().Msg("Stopped.")

	return nil
}

func main() {
	if err := runCommand(); err != nil {
		logger.Error().Err(err).Msg("Failed")
		os.Exit(1)
	}
}
