package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func SendTokensCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-tokens [address] [amount]",
		Short: "Transfer tokens to specific address",
		Long:  "Transfer some amount of tokens to specific address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransfer(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)

	cmd.Flags().Var(
		&params.feeCredit,
		feeCreditFlag,
		"Fee credit",
	)

	cmd.Flags().StringArrayVar(&params.currencies,
		tokenFlag,
		nil,
		"Token to transfer in format '<currencyId>=<amount>', can be used multiple times",
	)

	return cmd
}

func runTransfer(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Value
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	currencies, err := common.ParseCurrencies(params.currencies)
	if err != nil {
		return err
	}

	msgHash, err := service.RunContract(cfg.Address, nil, types.Value{}, params.feeCredit, amount, currencies, address)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}

	fmt.Println(msgHash)
	return nil
}
