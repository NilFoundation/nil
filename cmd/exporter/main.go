package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/exporter"
	"github.com/NilFoundation/nil/exporter/clickhouse"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

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

func main() {
	logger := logging.NewLogger("exporter")

	cobra.OnInitialize(initConfig)
	rootCmd := &cobra.Command{
		Use:   "exporter [-c config.yaml] [flags]",
		Short: "Exporter is a tool to export data from Nil blockchain to Clickhouse.",
		Long: `Exporter is a tool to export data from Nil blockchain to Clickhouse.
You could config it via config file or flags or environment variables.`,
		Run: func(cmd *cobra.Command, args []string) {
			requiredParams := []string{"clickhouse-endpoint", "clickhouse-login", "clickhouse-password", "clickhouse-database"}
			absentParams := make([]string, 0)
			for _, param := range requiredParams {
				if viper.GetString(param) == "" {
					absentParams = append(absentParams, param)
				}
			}
			if len(absentParams) > 0 {
				var buffer bytes.Buffer
				cmd.SetOut(&buffer)

				check.PanicIfErr(cmd.Help())

				fmt.Printf("Required parameters are absent: %v\n%s", absentParams, buffer.String())
				os.Exit(1)
			}
		},
	}

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

	if onlySchemeInit {
		clickhouseExporter, err := clickhouse.NewClickhouseDriver(ctx, clickhouseEndpoint, clickhouseLogin, clickhousePassword, clickhouseDatabase)
		check.PanicIfErr(err)

		check.PanicIfErr(clickhouseExporter.SetupScheme(ctx))

		logger.Info().Msg("Scheme initialized")
		return
	}

	var clickhouseExporter *clickhouse.ClickhouseDriver
	for {
		clickhouseDriver, err := clickhouse.NewClickhouseDriver(ctx, clickhouseEndpoint, clickhouseLogin, clickhousePassword, clickhouseDatabase)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create Clickhouse driver")
			time.Sleep(3 * time.Second)
			continue
		}
		clickhouseExporter = clickhouseDriver
		break
	}

	cfg := &exporter.Cfg{
		APIEndpoints:   []string{apiEndpoint},
		ExporterDriver: clickhouseExporter,
		BlocksChan:     make(chan *exporter.BlockMsg, 100),
		ErrorChan:      make(chan error),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case errMsg := <-cfg.ErrorChan:
				logger.Error().Err(errMsg).Msg("Error occurred")
				if strings.Contains(errMsg.Error(), "read: connection reset by peer") {
					err := cfg.ExporterDriver.Reconnect()
					if err != nil {
						logger.Error().Err(errMsg).Msg("Failed to reconnect")
					}
				}
			}
		}
	}()

	check.PanicIfErr(exporter.StartExporter(ctx, cfg))
	logger.Info().Msg("Exporter stopped")
}
