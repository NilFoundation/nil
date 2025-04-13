package smartaccount

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func SendTokensCommand(cfg *common.Config) *cobra.Command {
	params := &smartAccountParams{
		Params: &common.Params{},
	}

	cmd := &cobra.Command{
		Use:   "send-tokens [address] [amount]",
		Short: "Transfer tokens to a specific address",
		Long:  "Transfer some amount of tokens to a specific address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransfer(cmd, args, cfg, params)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	cmd.Flags().Var(
		&params.Fee.FeeCredit,
		feeCreditFlag,
		"The fee credit for processing the transfer",
	)

	cmd.Flags().StringArrayVar(&params.tokens,
		tokenFlag,
		nil,
		"The custom tokens to transfer in as a map 'tokenId=amount', can be set multiple times",
	)

	return cmd
}

func runTransfer(cmd *cobra.Command, args []string, cfg *common.Config, params *smartAccountParams) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Value
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	tokens, err := common.ParseTokens(params.tokens)
	if err != nil {
		return err
	}

	txnHash, err := service.RunContract(
		cfg.Address, nil, types.NewFeePackFromFeeCredit(params.Fee.FeeCredit), amount, tokens, address)
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
