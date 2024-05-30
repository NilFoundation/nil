package solc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
)

func CompileSource(sourcePath string) (map[string]*compiler.Contract, error) {
	solc, err := exec.LookPath("solc")
	if err != nil {
		return nil, fmt.Errorf("solc compiler not found: %w", err)
	}

	cmd := exec.Command(solc, "--combined-json", "abi,bin", sourcePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute `%s`: %w", cmd, err)
	}

	// Provide empty strings for the additional required arguments
	contracts, err := compiler.ParseCombinedJSON(output, "" /*source*/, "" /*langVersion*/, "" /*compilerVersion*/, "" /*compilerOpts*/)
	if err != nil {
		return nil, fmt.Errorf("failed to parse solc output: %w", err)
	}

	res := make(map[string]*compiler.Contract)
	for name, c := range contracts {
		// extract contract name
		parts := strings.Split(name, ":")
		res[parts[len(parts)-1]] = c
	}

	return res, nil
}

func ExtractABI(c *compiler.Contract) abi.ABI {
	data, err := json.Marshal(c.Info.AbiDefinition)
	if err != nil {
		panic(fmt.Errorf("failed to extract abi: %w", err))
	}

	abi, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Errorf("failed to extract abi: %w", err))
	}
	return abi
}
