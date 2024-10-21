package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/cmd/exporter/internal"
	"github.com/NilFoundation/nil/nil/cmd/exporter/internal/clickhouse"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger  = logging.NewLogger("exporter")
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

		// Search config in home directory with the name "exporter.cobra" (without an extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("exporter")
	}

	check.PanicIfErr(viper.ReadInConfig())

	viper.AutomaticEnv()
}

// This makes the --help command exit instead of proceeding to run
// the exporter
func ApplyExitOnHelp(c *cobra.Command, exitCode int) {
	helpFunc := c.HelpFunc()
	c.SetHelpFunc(func(c *cobra.Command, s []string) {
		helpFunc(c, s)
		os.Exit(exitCode)
	})
}

func main() {
	logging.SetLogSeverityFromEnv()
	logging.SetupGlobalLogger("info")

	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:   "exporter [-c config.yaml] [flags]",
		Short: "Exporter is a tool to export data from Nil blockchain to Clickhouse.",
		Long: `Exporter is a tool to export data from Nil blockchain to Clickhouse.
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
	ApplyExitOnHelp(rootCmd, 0)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $CWD/exporter.cobra.yaml)")
	rootCmd.Flags().StringP("api-endpoint", "a", "http://127.0.0.1:8529", "API endpoint")
	rootCmd.Flags().StringP("clickhouse-endpoint", "e", "127.0.0.1:9000", "Clickhouse endpoint")
	rootCmd.Flags().StringP("clickhouse-login", "l", "", "Clickhouse login")
	rootCmd.Flags().StringP("clickhouse-password", "p", "", "Clickhouse password")
	rootCmd.Flags().StringP("clickhouse-database", "d", "", "Clickhouse database")
	rootCmd.Flags().BoolP("only-scheme-init", "s", false, "Only scheme initialization")

	check.PanicIfErr(viper.BindPFlags(rootCmd.Flags()))

	check.PanicIfErr(rootCmd.Execute())

	clickhousePassword := viper.GetString("clickhouse-password")
	clickhouseEndpoint := viper.GetString("clickhouse-endpoint")
	clickhouseLogin := viper.GetString("clickhouse-login")
	clickhouseDatabase := viper.GetString("clickhouse-database")
	apiEndpoint := viper.GetString("api-endpoint")
	onlySchemeInit := viper.GetBool("only-scheme-init")

	ctx := context.Background()

	clickhouseExporter, err := clickhouse.NewClickhouseDriver(ctx, clickhouseEndpoint, clickhouseLogin, clickhousePassword, clickhouseDatabase)
	check.PanicIfErr(err)

	if onlySchemeInit {
		check.PanicIfErr(clickhouseExporter.SetupScheme(ctx))

		logger.Info().Msg("Scheme initialized")
		return
	}

	check.PanicIfErr(internal.StartExporter(ctx, &internal.Cfg{
		Client:         rpc.NewClient(apiEndpoint, logger),
		ExporterDriver: clickhouseExporter,
		BlocksChan:     make(chan *internal.BlockWithShardId, 1000),
	}))

	logger.Info().Msg("Exporter stopped")
}
