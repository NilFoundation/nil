package minter

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func MintCurrencyCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mint-currency [address] [amount]",
		Short: "Mint wallet/contract currency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMintCurrency(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(&params.withdraw, withdrawFlag, false, "Withdraw minted currency to the wallet/contract")

	return cmd
}

func runMintCurrency(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Value
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	txHash, err := service.CurrencyMint(address, amount, params.withdraw)
	if err != nil {
		return err
	}
	if !config.Quiet {
		fmt.Printf("Minted %v amount of currency to %v, TX Hash: ", amount, address)
	}
	fmt.Println(txHash)
	return nil
}
