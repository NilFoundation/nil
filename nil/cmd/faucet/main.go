package main

import (
	"context"
	"fmt"
	"os"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/spf13/cobra"
)

type Command uint

const (
	CommandRun Command = iota + 1
)

type config struct {
	command  Command
	port     int
	endpoint string
}

func main() {
	cfg := parseArgs()

	if cfg.command != CommandRun {
		fmt.Printf("Faucet failed: unknown command\n")
		os.Exit(1)
	}

	if err := processRun(cfg); err != nil {
		fmt.Printf("Faucet failed: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func processRun(cfg *config) error {
	addr := fmt.Sprintf("tcp://127.0.0.1:%d", cfg.port)
	client := rpc_client.NewClient(cfg.endpoint, logging.NewLogger("faucet"))

	serviceFaucet, err := faucet.NewService(client)
	if err != nil {
		return err
	}
	return serviceFaucet.Run(context.Background(), addr)
}

func parseArgs() *config {
	cfg := &config{}
	rootCmd := &cobra.Command{
		Use:           "faucet [global flags] [command]",
		Short:         "faucet server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().StringVar(&cfg.endpoint, "node-endpoint", "http://127.0.0.1:8529", "nil node endpoint")
	rootCmd.PersistentFlags().IntVar(&cfg.port, "port", 8527, "http service port")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run faucet server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.command = CommandRun
		},
	}
	rootCmd.AddCommand(runCmd)

	logLevel := rootCmd.PersistentFlags().StringP(
		"log-level",
		"l",
		"info",
		"log level: trace|debug|info|warn|error|fatal|panic")
	logging.SetupGlobalLogger(*logLevel)

	check.PanicIfErr(rootCmd.Execute())

	return cfg
}
