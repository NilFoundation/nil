package main

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/commands"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/spf13/cobra"
)

const appTitle = "=nil; Sync Committee CLI"

func main() {
	check.PanicIfNotCancelledErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run Sync Committee CLI Tool",
	}

	logging.SetupGlobalLogger("info")
	logger := logging.NewLogger("sync_committee_cli")

	builders := []interface {
		Build() (*cobra.Command, error)
	}{
		commands.NewGetTasksCmd(logger),
		commands.NewGetTaskTreeCmd(logger),
		commands.NewDecodeBatchCmd(logger),
		commands.NewRollbackStateCmd(logger),
	}

	for _, builder := range builders {
		command, err := builder.Build()
		if err != nil {
			return fmt.Errorf("failed to build command: %w", err)
		}
		rootCmd.AddCommand(command)
	}

	versionCmd := cobrax.VersionCmd(appTitle)
	rootCmd.AddCommand(versionCmd)
	return rootCmd.Execute()
}
