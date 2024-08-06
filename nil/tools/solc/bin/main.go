package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	logger := logging.NewLogger("solc")

	cmd := &cobra.Command{
		Short: "Tool for solidity contracts compilation",
		Long:  "For each contract in solidity source this tool will output two files (code-hex and abi) with corresponding names",
	}

	cmd.Flags().StringP("source", "s", "contract.sol", "path to solidity source file")
	cmd.Flags().StringP("output", "o", "", "path to output dir. leave empty to use source dir")
	cmd.Flags().StringP("contract", "c", "", "particular contract to compile. leave empty to compile all contracts")

	check.PanicIfErr(viper.BindPFlags(cmd.Flags()))

	check.PanicIfErr(cmd.Execute())

	sourcePath := viper.GetString("source")
	contractName := viper.GetString("contract")
	contractDir := viper.GetString("output")
	if contractDir == "" {
		contractDir = filepath.Dir(sourcePath)
	}
	contractDir, err := filepath.Abs(contractDir)
	check.PanicIfErr(err)
	check.PanicIfErr(os.MkdirAll(contractDir, os.ModePerm))

	contracts, err := solc.CompileSource(sourcePath)
	check.PanicIfErr(err)

	for name, c := range contracts {
		if contractName != "" && contractName != name {
			continue
		}
		abiFile := filepath.Join(contractDir, name+".abi")
		codeFile := filepath.Join(contractDir, name+".bin")

		abi, err := json.Marshal(c.Info.AbiDefinition)
		check.PanicIfErr(err)

		check.LogAndPanicIfErrf(os.WriteFile(abiFile, abi, 0o644), logger, "failed to write abi for contract %s", name) //nolint:gosec

		check.LogAndPanicIfErrf(os.WriteFile(codeFile, []byte(c.Code), 0o644), logger, "failed to write code hext for contract %s", name) //nolint:gosec
	}
}
