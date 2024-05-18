package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/rpc"
	"github.com/rs/zerolog/log"
)

func main() {
	// read argument
	path := flag.String("path", "models_bel.ts", "path to write typescript types")
	flag.Parse()

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
