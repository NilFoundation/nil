package main

import (
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc"
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

	logger := logging.NewLogger("ts-generate")

	// get the absolute path
	absPath, err := filepath.Abs(*path)
	check.PanicIfErr(err)

	// open the file
	openFile, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY, 0o644)
	check.PanicIfErr(err)

	typescriptContent, err := rpc.ExportTypescriptTypes()
	check.PanicIfErr(err)

	_, err = openFile.Write(typescriptContent)
	check.PanicIfErr(err)

	logger.Info().Msgf("Export Typescript Types to %s", absPath)
}
