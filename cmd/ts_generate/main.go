package main

import (
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/rpc"
	"github.com/rs/zerolog/log"
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

	// get the absolute path
	absPath, err := filepath.Abs(*path)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get absolute path")
	}
	// open the file
	openFile, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open file")
	}
	typescriptContent, err := rpc.ExportTypescriptTypes()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to export typescript types")
	}
	_, err = openFile.Write(typescriptContent)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to write to file %s", absPath)
	}

	log.Info().Msgf("Export Typescript Types to %s", absPath)
}
