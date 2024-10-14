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
)

type Command uint

const (
	CommandRun Command = iota
)

type config struct {
	command  Command
	dbPath   string
	endpoint string
}

func main() {
	cfg := parseArgs()

	if cfg.command != CommandRun {
		fmt.Printf("Cometa failed: unknown command\n")
		os.Exit(1)
	}

	if err := processRun(cfg); err != nil {
		fmt.Printf("Cometa failed: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func createRpcClient(cfg *config) *rpc.Client {
	return rpc.NewClientWithDefaultHeaders(
		cfg.endpoint,
		zerolog.Nop(),
		map[string]string{
			"User-Agent": "cometa/" + version.GetGitRevision(),
		},
	)
}

func processRun(cfg *config) error {
	client := createRpcClient(cfg)
	service, err := cometa.NewService(cfg.dbPath, client)
	if err != nil {
		return err
	}
	return service.Run(context.Background(), "tcp://127.0.0.1:8528")
}

func parseArgs() *config {
	cfg := &config{}
	rootCmd := &cobra.Command{
		Use:           "cometa [global flags] [command]",
		Short:         "cometa contracts metadata app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().StringVar(&cfg.endpoint, "node-endpoint", "http://127.0.0.1:8529", "nil node endpoint")
	rootCmd.PersistentFlags().StringVar(&cfg.dbPath, "dbpath", "cometa.db", "path where to store database")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run cometa server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.command = CommandRun
		},
	}
	rootCmd.AddCommand(runCmd)

	logLevel := rootCmd.PersistentFlags().StringP("log-level", "l", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	logging.SetupGlobalLogger(*logLevel)

	check.PanicIfErr(rootCmd.Execute())

	return cfg
}
