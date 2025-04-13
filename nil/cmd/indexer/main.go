package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/NilFoundation/nil/nil/services/indexer"
	"github.com/NilFoundation/nil/nil/services/indexer/clickhouse"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger  = logging.NewLogger("indexer")
	cfgFile string
)

func initConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.Getwd()
		check.PanicIfErr(err)

		// Search config in home directory with the name "indexer.cobra" (without an extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("indexer")
	}

	check.PanicIfErr(viper.ReadInConfig())

	viper.AutomaticEnv()
}

func main() {
	logging.SetLogSeverityFromEnv()
	logging.SetupGlobalLogger("info")

	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:   "indexer [-c config.yaml] [flags]",
		Short: "indexer is a tool to export data from Nil blockchain to Clickhouse.",
		Long: `Indexer is a tool to export data from Nil blockchain to Clickhouse.
You could config it via config file or flags or environment variables.`,
		Run: func(cmd *cobra.Command, args []string) {
			requiredParams := []string{"clickhouse-endpoint", "clickhouse-login", "clickhouse-database"}
			absentParams := make([]string, 0)
			for _, param := range requiredParams {
				if viper.GetString(param) == "" {
					absentParams = append(absentParams, param)
				}
			}
			if len(absentParams) > 0 {
				var buffer bytes.Buffer
				cmd.SetOut(&buffer)

				fmt.Printf("Required parameters are absent: %v\n%s", absentParams, buffer.String())
				os.Exit(1)
			}
		},
	}
	cobrax.ExitOnHelp(rootCmd)

	rootCmd.PersistentFlags().StringVarP(
		&cfgFile,
		"config",
		"c",
		"",
		"config file (default is $CWD/indexer.cobra.yaml)")
	rootCmd.Flags().StringP("api-endpoint", "a", "http://127.0.0.1:8529", "API endpoint")
	rootCmd.Flags().StringP("clickhouse-endpoint", "e", "127.0.0.1:9000", "Clickhouse endpoint")
	rootCmd.Flags().StringP("clickhouse-login", "l", "", "Clickhouse login")
	rootCmd.Flags().StringP("clickhouse-password", "p", "", "Clickhouse password")
	rootCmd.Flags().StringP("clickhouse-database", "d", "", "Clickhouse database")
	rootCmd.Flags().Bool("allow-db-clear", false, "Drop db if versions differ")
	rootCmd.Flags().Bool("index-txpool", false, "Do indexing of txpool")

	check.PanicIfErr(viper.BindPFlags(rootCmd.Flags()))

	check.PanicIfErr(rootCmd.Execute())

	clickhousePassword := viper.GetString("clickhouse-password")
	clickhouseEndpoint := viper.GetString("clickhouse-endpoint")
	clickhouseLogin := viper.GetString("clickhouse-login")
	clickhouseDatabase := viper.GetString("clickhouse-database")
	apiEndpoint := viper.GetString("api-endpoint")
	allowDbDrop := viper.GetBool("allow-db-clear")

	ctx := context.Background()

	clickhouseDriver, err := clickhouse.NewClickhouseDriver(
		ctx, clickhouseEndpoint, clickhouseLogin, clickhousePassword, clickhouseDatabase)
	check.PanicIfErr(err)

	check.PanicIfErr(indexer.StartIndexer(ctx, &indexer.Cfg{
		Client:        rpc.NewClient(apiEndpoint, logger),
		IndexerDriver: clickhouseDriver,
		AllowDbDrop:   allowDbDrop,
		DoIndexTxpool: viper.GetBool("index-txpool"),
	}))

	logger.Info().Msg("Indexer stopped")
}
