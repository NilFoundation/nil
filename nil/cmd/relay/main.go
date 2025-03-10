package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/cobrax"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/spf13/cobra"
)

type config struct {
	LogLevel  string `yaml:"logLevel,omitempty"`
	PprofPort int    `yaml:"pprofPort,omitempty"`

	Network   *network.Config   `yaml:"network,omitempty"`
	Telemetry *telemetry.Config `yaml:"telemetry,omitempty"`
}

func parseArgs() *config {
	cfg := &config{
		Network:   network.NewDefaultConfig(),
		Telemetry: telemetry.NewDefaultConfig(),
	}
	cfg.Network.Relay = true

	check.PanicIfErr(cobrax.LoadConfigFromFile(cobrax.GetConfigNameFromArgs(), cfg))

	rootCmd := &cobra.Command{
		Use:           "relay [flags]",
		Short:         "relay for nild cluster network",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cobrax.AddConfigFlag(rootCmd.Flags())
	cobrax.AddLogLevelFlag(rootCmd.Flags(), &cfg.LogLevel)
	cobrax.AddPprofPortFlag(rootCmd.Flags(), &cfg.PprofPort)
	network.AddFlags(rootCmd.Flags(), cfg.Network)
	telemetry.AddFlags(rootCmd.Flags(), cfg.Telemetry)

	rootCmd.AddCommand(cobrax.VersionCmd("relay"))

	check.PanicIfErr(rootCmd.Execute())

	return cfg
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg := parseArgs()

	logging.SetupGlobalLogger(cfg.LogLevel)
	profiling.Start(cfg.PprofPort)

	check.PanicIfErr(telemetry.Init(ctx, cfg.Telemetry))
	defer telemetry.Shutdown(ctx)

	nm, err := network.NewManager(ctx, cfg.Network)
	check.PanicIfErr(err)
	defer nm.Close()

	<-ctx.Done()
}
