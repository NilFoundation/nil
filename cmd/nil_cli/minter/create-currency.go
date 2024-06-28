package minter

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
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
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Uint256
	if err := amount.SetFromDecimal(args[1]); err != nil {
		return err
	}

	name := args[2]

	return service.CurrencyCreate(address, amount.ToBig(), name, params.withdraw)
}
