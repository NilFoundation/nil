package main

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/journald_forwarder"
	"github.com/spf13/cobra"
)

func main() {
	cfg := &journald_forwarder.Config{}
	logger := logging.NewLogger("journald_forwarder")
	rootCmd := &cobra.Command{
		Use:   "journald_to_click",
		Short: "Run journald to click handler",
	}

	rootCmd.Flags().StringVar(&cfg.ClickhouseAddr, "clickhouse-endpoint", "127.0.0.1:9000", "clickhouse endpoint")
	rootCmd.Flags().StringVar(&cfg.ListenAddr, "own-endpoint", "127.0.0.1:5678", "own rpc endpoint")
	rootCmd.Flags().StringVar(&cfg.DbUser, "db-user", "nil", "database user")
	rootCmd.Flags().StringVar(&cfg.DbDatabase, "db-database", "default", "database name")
	rootCmd.Flags().StringVar(&cfg.DbPassword, "db-password", "", "database password")

	check.PanicIfErr(rootCmd.Execute())
	check.PanicIfErr(journald_forwarder.Run(context.Background(), *cfg, logger))
}
