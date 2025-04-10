package cometa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cometa [options]",
		Short: "Interact with the Cometa service",
	}
	cmd.AddCommand(GetInfoCommand())
	cmd.AddCommand(GetRegisterCommand())

	return cmd
}

func GetRegisterCommand() *cobra.Command {
	params := &cometaParams{}

	cmd := &cobra.Command{
		Use:   "register [options] address",
		Short: "Register contract metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterCommand(cmd, params)
		},
	}

	cmd.Flags().Var(&params.address, "address", "The contract address")
	cmd.Flags().StringVar(&params.inputJsonFile, "compile-input", "", "The JSON file with the compilation input")
	if err := cmd.MarkFlagRequired("compile-input"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return cmd
}

func GetInfoCommand() *cobra.Command {
	params := &cometaParams{}

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Acquire the metadata for a contract",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfoCommand(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.saveToFile, "save-to", "", "Save the metadata to a file")
	cmd.Flags().Var(&params.address, "address", "The contract address")
	if err := cmd.MarkFlagRequired("address"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return cmd
}

func runRegisterCommand(_ *cobra.Command, params *cometaParams) error {
	cometaClient := common.GetCometaRpcClient()

	inputJsonData, err := os.ReadFile(params.inputJsonFile)
	if err != nil {
		return fmt.Errorf("failed to read the input JSON file: %w", err)
	}

	inputJson, err := normalizeCompileInput(string(inputJsonData), params.inputJsonFile)
	if err != nil {
		return fmt.Errorf("failed to normalize the input JSON file: %w", err)
	}

	err = cometaClient.RegisterContract(inputJson, params.address)
	if err != nil {
		return fmt.Errorf("failed to register the contract: %w", err)
	}

	fmt.Printf("Contract metadata for address %s has been registered\n", params.address)

	return nil
}

func normalizeCompileInput(inputJson, inputJsonFile string) (string, error) {
	var input cometa.CompilerTask
	if err := json.Unmarshal([]byte(inputJson), &input); err != nil {
		return "", fmt.Errorf("failed to unmarshal the input JSON file: %w", err)
	}
	if input.BasePath == "" {
		input.BasePath = filepath.Dir(inputJsonFile)
	}
	if err := input.Normalize(filepath.Dir(inputJsonFile)); err != nil {
		return "", fmt.Errorf("failed to normalize the input JSON file: %w", err)
	}
	data, err := json.MarshalIndent(input, "", "  ")
	return string(data), err
}

func runInfoCommand(_ *cobra.Command, params *cometaParams) error {
	cometa := common.GetCometaRpcClient()

	contract, err := cometa.GetContract(params.address)
	if err != nil {
		return fmt.Errorf("failed to get the contract: %w", err)
	}

	if len(params.saveToFile) > 0 {
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal contract metadata to JSON: %w", err)
		}
		if err = os.WriteFile(params.saveToFile, data, 0o600); err != nil {
			return fmt.Errorf("failed to save metadata to a file: %w", err)
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
