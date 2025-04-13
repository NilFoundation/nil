package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetSendExternalTransactionCommand(cfg *common.Config) *cobra.Command {
	params := &contractParams{
		Params: &common.Params{},
	}

	cmd := &cobra.Command{
		Use:   "send-external-transaction [address] [bytecode or method] [args...]",
		Short: "Send an external transaction to a smart contract",
		Long:  "Send an external transaction to the smart contract with the specified bytecode or command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSendExternalTransaction(cmd, args, cfg, params)
		},
		SilenceUsage: true,
		// This command is useful for only rare cases, so it's hidden
		// to avoid confusion for the users between "send" and "send-transaction"
		Hidden: true,
	}

	cmd.Flags().StringVar(
		&params.AbiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	cmd.Flags().BoolVar(
		&params.noSign,
		noSignFlag,
		false,
		"Define whether the external transaction should be signed",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	return cmd
}

func runSendExternalTransaction(cmd *cobra.Command, args []string, cfg *common.Config, params *contractParams) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	abi, err := common.ReadAbiFromFile(params.AbiPath)
	if err != nil {
		return err
	}

	calldata, err := common.PrepareArgs(abi, args[1], args[2:])
	if err != nil {
		return err
	}

	txnHash, err := service.SendExternalTransaction(calldata, address, params.noSign)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(txnHash); err != nil {
			return err
		}
	}

	if !common.Quiet {
		fmt.Print("Transaction hash: ")
	}
	fmt.Println(txnHash)
	return nil
}
