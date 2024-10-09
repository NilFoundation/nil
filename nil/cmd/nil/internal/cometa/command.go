package cometa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cometa [options]",
		Short: "Interact with Cometa service",
	}
	cmd.AddCommand(GetInfoCommand())
	cmd.AddCommand(GetDeployCommand())

	return cmd
}

func GetDeployCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register [options] address",
		Short: "Register contract metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterCommand(cmd)
		},
	}

	cmd.Flags().Var(&params.address, "address", "Contract address")
	cmd.Flags().StringVar(&params.inputJsonFile, "compile-input", "", "Compilation input JSON file")
	if err := cmd.MarkFlagRequired("compile-input"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return cmd
}

func GetInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get contract's metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfoCommand(cmd)
		},
	}

	cmd.Flags().StringVar(&params.saveToFile, "save-to", "", "Save metadata to file")
	cmd.Flags().Var(&params.address, "address", "Contract address")
	if err := cmd.MarkFlagRequired("address"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return cmd
}

func runRegisterCommand(_ *cobra.Command) error {
	cometaClient := common.GetCometaRpcClient()

	inputJsonData, err := os.ReadFile(params.inputJsonFile)
	if err != nil {
		return fmt.Errorf("failed to read input JSON file: %w", err)
	}

	inputJson, err := normalizeCompileInput(string(inputJsonData), params.inputJsonFile)
	if err != nil {
		return fmt.Errorf("failed to normalize input JSON: %w", err)
	}

	contractData, err := cometaClient.CompileContract(inputJson)
	if err != nil {
		return fmt.Errorf("failed to compile contract: %w", err)
	}

	if err := cometaClient.RegisterContract(contractData, params.address); err != nil {
		return fmt.Errorf("failed to register contract: %w", err)
	}

	fmt.Printf("Contract metadata for address %s has been registered\n", params.address)

	return nil
}

func normalizeCompileInput(inputJson, inputJsonFile string) (string, error) {
	var input cometa.JsonInput
	if err := json.Unmarshal([]byte(inputJson), &input); err != nil {
		return "", fmt.Errorf("failed to unmarshal input json: %w", err)
	}
	if input.BasePath == "" {
		input.BasePath = filepath.Dir(inputJsonFile)
	}
	if err := input.Normalize(); err != nil {
		return "", fmt.Errorf("failed to normalize input json: %w", err)
	}
	data, err := json.MarshalIndent(input, "", "  ")
	return string(data), err
}

func runInfoCommand(_ *cobra.Command) error {
	cometa := common.GetCometaRpcClient()

	contract, err := cometa.GetContract(params.address)
	if err != nil {
		return fmt.Errorf("failed to get contract: %w", err)
	}

	if len(params.saveToFile) > 0 {
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal contract metadata to JSON: %w", err)
		}
		if err = os.WriteFile(params.saveToFile, data, 0o600); err != nil {
			return fmt.Errorf("failed to save metadata to file: %w", err)
		}
		fmt.Printf("Contract metadata for address %s has been saved to file '%s'\n", params.address, params.saveToFile)
	} else {
		fmt.Printf("Contract metadata for address %s\n", params.address)
		fmt.Printf("  Name: %s\n", contract.Name)
		if len(contract.Description) > 0 {
			fmt.Printf("  Description:\n%s\n", contract.Description)
		}
		fmt.Printf("  Source files: [")
		sep := ""
		for name := range contract.SourceCode {
			fmt.Print(sep + name)
			sep = ", "
		}
		fmt.Printf("]\n")
		fmt.Printf("  Bytecode size: %d\n", len(contract.Code))
	}

	return nil
}
