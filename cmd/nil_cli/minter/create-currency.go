package minter

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func CreateCurrencyCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-currency [address] [amount] [name]",
		Short: "Create wallet/contract currency",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateCurrency(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(&params.withdraw, withdrawFlag, false, "Withdraw created currency to the wallet/contract")

	return cmd
}

func runCreateCurrency(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Value
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	name := args[2]

	currencyId, err := service.CurrencyCreate(address, amount, name, params.withdraw)
	if err != nil {
		return err
	}
	if !config.Quiet {
		fmt.Print("Created Currency ID: ")
	}
	fmt.Println(currencyId)
	return nil
}
