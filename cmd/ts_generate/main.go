package main

import (
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/rpc"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ts_generate [output file]",
		Short: "Generate typescript types for RPC API",
	}

	// read argument
	path := rootCmd.Flags().StringP("output", "o", "rpc.ts", "Output file path")
	cmdErr := rootCmd.Execute()

	if cmdErr != nil {
		return
	}

	logger := common.NewLogger("ts-generate")

	// get the absolute path
	absPath, err := filepath.Abs(*path)
	common.FatalIf(err, logger, "Failed to get absolute path")

	// open the file
	openFile, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY, 0o644)
	common.FatalIf(err, logger, "Failed to open file")

	typescriptContent, err := rpc.ExportTypescriptTypes()
	common.FatalIf(err, logger, "Failed to export typescript types")

	_, err = openFile.Write(typescriptContent)
	common.FatalIf(err, logger, "Failed to write to file %s", absPath)

	logger.Info().Msgf("Export Typescript Types to %s", absPath)
}
