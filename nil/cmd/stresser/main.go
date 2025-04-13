package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/services/stresser"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var logLevel *string

func main() {
	var configFile string
	var taskName string
	rootCmd := &cobra.Command{
		Use:   "stresser --config <config-file>",
		Short: "Run stresser",
		RunE: func(cmd *cobra.Command, args []string) error {
			level, err := zerolog.ParseLevel(*logLevel)
			check.PanicIfErr(err)
			zerolog.SetGlobalLevel(level)
			st, err := stresser.NewStresserFromFile(configFile, taskName)
			if err != nil {
				return fmt.Errorf("failed to create stresser: %w", err)
			}
			if err = st.Run(context.Background()); err != nil {
				return fmt.Errorf("failed to run stresser: %w", err)
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.Flags().StringVarP(&taskName, "task", "t", "", "task name")
	logLevel = rootCmd.Flags().StringP(
		"log-level",
		"l",
		"info",
		"log level: trace|debug|info|warn|error|fatal|panic")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
