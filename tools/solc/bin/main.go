package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	logger := common.NewLogger("solc")

	cmd := &cobra.Command{
		Short: "Tool for solidity contracts compilation",
		Long:  "For each contract in solidity source this tool will output two files (code-hex and abi) with corresponding names",
	}

	cmd.Flags().StringP("source", "s", "contract.sol", "path to solidity source file")
	cmd.Flags().StringP("contract", "c", "", "particular contract to compile. leave empty to compile all contracts")

	err := viper.BindPFlags(cmd.Flags())
	common.FatalIf(err, logger, "Failed to bind flags")

	err = cmd.Execute()
	common.FatalIf(err, logger, "Failed to parse args")

	sourcePath := viper.GetString("source")
	contractName := viper.GetString("contract")

	contracts, err := solc.CompileSource(sourcePath)
	common.FatalIf(err, logger, "failed to compile contract `%s`", sourcePath)

	contractDir := filepath.Dir(sourcePath)
	for name, c := range contracts {
		if contractName != "" && contractName != name {
			continue
		}
		abiFile := filepath.Join(contractDir, name+".abi")
		codeFile := filepath.Join(contractDir, name+".bin")

		abi, err := json.Marshal(c.Info.AbiDefinition)
		common.FatalIf(err, logger, "failed to marshal abi")

		common.FatalIf(os.WriteFile(abiFile, abi, 0o644), logger, "failed to write abi for contract %s", name) //nolint:gosec

		common.FatalIf(os.WriteFile(codeFile, []byte(c.Code), 0o644), logger, "failed to write code hext for contract %s", name) //nolint:gosec

		logger.Info().Str("file", abiFile).Msgf("ABI = %s", abi)
		logger.Info().Str("file", codeFile).Msgf("Code = %s", c.Code)
	}
}
