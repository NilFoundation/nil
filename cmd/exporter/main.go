package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get working directory")
		}

		// Search config in home directory with name "exporter.cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName("exporter")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Can't read config")
	}

	viper.AutomaticEnv()
}

func main() {
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
				if err := cmd.Help(); err != nil {
					log.Fatal().Err(err).Msg("Failed to print help")
				}
				fmt.Printf("Required parameters are absent: %v\n%s", absentParams, buffer.String())
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $CWD/exporter.cobra.yaml)")
	rootCmd.Flags().StringP("api-endpoint", "a", "http://127.0.0.1:8545", "API endpoint")
	rootCmd.Flags().StringP("clickhouse-endpoint", "e", "", "Clickhouse endpoint")
	rootCmd.Flags().StringP("clickhouse-login", "l", "", "Clickhouse login")
	rootCmd.Flags().StringP("clickhouse-password", "p", "", "Clickhouse password")
	rootCmd.Flags().StringP("clickhouse-database", "d", "", "Clickhouse database")
	rootCmd.Flags().BoolP("only-scheme-init", "s", false, "Only scheme initialization")

	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		log.Fatal().Err(err).Msg("Failed to bind flags")
	}

	if err := rootCmd.Execute(); err != nil {
		return
	}
	clickhousePassword := viper.GetString("clickhouse-password")
	clickhouseEndpoint := viper.GetString("clickhouse-endpoint")
	clickhouseLogin := viper.GetString("clickhouse-login")
	clickhouseDatabase := viper.GetString("clickhouse-database")
	apiEndpoint := viper.GetString("api-endpoint")
	onlySchemeInit := viper.GetBool("only-scheme-init")

	ctx := context.Background()

	if onlySchemeInit {
		clickhouseExporter, err := clickhouse.NewClickhouseDriver(ctx, clickhouseEndpoint, clickhouseLogin, clickhousePassword, clickhouseDatabase)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create Clickhouse driver")
		}
		if err = clickhouseExporter.SetupScheme(ctx); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Clickhouse scheme")
		}
		log.Info().Msg("Scheme initialized")
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

	cfg := exporter.Cfg{
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
				log.Error().Err(errMsg).Msg("Error occurred")
			}
		}
	}()

	if err := exporter.StartExporter(ctx, &cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to start exporter")
	}
	log.Info().Msg("Exporter stopped")
}
