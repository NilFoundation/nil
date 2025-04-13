package smartaccount

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func SendTransactionCommand(cfg *common.Config) *cobra.Command {
	params := &smartAccountParams{
		Params: &common.Params{},
	}

	cmd := &cobra.Command{
		Use:   "send-transaction [address] [bytecode or method] [args...]",
		Short: "Send a transaction to a smart contract via the smart account",
		Long:  "Send a transaction to the smart contract with the specified bytecode or command via the smart account",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSend(cmd, args, cfg, params)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(
		&params.AbiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"The amount of default tokens to send",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	cmd.Flags().Var(
		&params.Fee.FeeCredit,
		feeCreditFlag,
		"The fee credit for transaction processing",
	)

	cmd.Flags().StringArrayVar(&params.tokens,
		tokenFlag,
		nil,
		"The custom tokens to transfer in as a map 'tokenId=amount', can be set multiple times",
	)

	cmd.Flags().StringVar(&params.priorityFee,
		priorityFee,
		"0",
		"The priority fee for the message",
	)

	return cmd
}

func runSend(cmd *cobra.Command, args []string, cfg *common.Config, params *smartAccountParams) error {
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

	tokens, err := common.ParseTokens(params.tokens)
	if err != nil {
		return err
	}

	txnHash, err := service.RunContract(
		cfg.Address, calldata, types.NewFeePackFromFeeCredit(params.Fee.FeeCredit), params.amount, tokens, address)
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
