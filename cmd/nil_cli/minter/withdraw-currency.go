package minter

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func WithdrawCurrencyCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw-currency [address] [amount] [to_address]",
		Short: "Withdraw wallet/contract currency from minter to another address",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithdrawCurrency(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runWithdrawCurrency(_ *cobra.Command, args []string, cfg *common.Config) error {
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

	var to types.Address
	if err := to.Set(args[2]); err != nil {
		return err
	}

	return service.CurrencyWithdraw(address, amount.ToBig(), to)
}
