package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func TopUpCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-up [amount] [currency-id]",
		Short: "Top up wallet",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopUp(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runTopUp(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var amount types.Value
	if err := amount.Set(args[0]); err != nil {
		return err
	}

	faucetAddress := types.FaucetAddress
	currId := types.GetCurrencyName(types.CurrencyId(faucetAddress))
	if len(args) > 1 {
		var ok bool
		currId = args[1]
		currencies := types.GetCurrencies()
		faucetAddress, ok = currencies[currId]
		if !ok {
			if err := faucetAddress.Set(currId); err != nil {
				return fmt.Errorf("undefined currency id: %s", currId)
			}
		}
	}

	if _, err := service.GetBalance(cfg.Address); err != nil {
		return err
	}

	if err := service.TopUpViaFaucet(faucetAddress, cfg.Address, amount); err != nil {
		return err
	}

	var balance types.Value
	var err error
	if faucetAddress == types.FaucetAddress {
		balance, err = service.GetBalance(cfg.Address)
		if err != nil {
			return err
		}
	} else {
		currencies, err := service.GetCurrencies(cfg.Address)
		if err != nil {
			return err
		}
		var ok bool
		balance, ok = currencies[types.CurrencyId(faucetAddress)]
		if !ok {
			return fmt.Errorf("Currency %s for account %s is not found", faucetAddress, cfg.Address)
		}
	}

	if !config.Quiet {
		fmt.Print("Wallet balance: ")
	}

	fmt.Print(balance)
	if !config.Quiet && len(currId) > 0 {
		fmt.Printf(" [%s]", currId)
	}
	fmt.Println()

	return nil
}
