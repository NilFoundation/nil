package main

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/nil_load_generator"
	"github.com/spf13/cobra"
)

func main() {
	cfg := nil_load_generator.NewDefaultConfig()
	componentName := "nil-load-generator"
	logger := logging.NewLogger(componentName)
	rootCmd := &cobra.Command{
		Use:   componentName,
		Short: "Run nil load generator",
	}

	rootCmd.Flags().StringVar(&cfg.Endpoint, "endpoint", cfg.Endpoint, "rpc endpoint")
	rootCmd.Flags().StringVar(&cfg.OwnEndpoint, "own-endpoint", cfg.OwnEndpoint, "own rpc endpoint")
	rootCmd.Flags().StringVar(&cfg.FaucetEndpoint, "faucet-endpoint", cfg.FaucetEndpoint, "faucet rpc endpoint")
	rootCmd.Flags().Uint32Var(
		&cfg.CheckBalance, "check-balance", cfg.CheckBalance, "frequency of balance check in iterations")
	rootCmd.Flags().Uint32Var(
		&cfg.SwapPerIteration, "swap-per-iteration", cfg.SwapPerIteration, "amount of swaps per iteration")
	rootCmd.Flags().BoolVar(&cfg.Metrics, "metrics", cfg.Metrics, "export metrics via grpc")
	rootCmd.Flags().Var(&cfg.MintTokenAmount0, "mint-token-amount0", "mint amount for token0")
	rootCmd.Flags().Var(&cfg.MintTokenAmount1, "mint-token-amount1", "mint amount for token1")
	rootCmd.Flags().Var(&cfg.ThresholdAmount, "threshold-amount", "threshold amount")
	rootCmd.Flags().Var(&cfg.SwapAmount, "swap-amount", "swap amount")
	rootCmd.Flags().Var(&cfg.RpcSwapLimit, "rpc-swap-limit", "rpc swap limit")
	rootCmd.Flags().Uint32Var(
		&cfg.UniswapAccounts, "rpc-uniswap-accounts", cfg.UniswapAccounts, "number of uniswap accounts")
	rootCmd.Flags().StringVar(
		&cfg.LogLevel, "log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	rootCmd.Flags().DurationVar(
		&cfg.WaitClusterStartup, "wait-cluster-startup", cfg.WaitClusterStartup, "time to wait for cluster startup")

	check.PanicIfErr(rootCmd.Execute())

	if err := telemetry.Init(
		context.Background(),
		&telemetry.Config{ServiceName: componentName, ExportMetrics: cfg.Metrics},
	); err != nil {
		logger.Err(err).Send()
		panic("Can't init telemetry")
	}
	defer telemetry.Shutdown(context.Background())

	if err := nil_load_generator.Run(context.Background(), cfg, logger); err != nil {
		logger.Error().Err(err).Msg("Error during nil load generator run")
		panic(err)
	}
}
