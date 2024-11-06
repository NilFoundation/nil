package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetCurrenciesCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "currencies [address]",
		Short: "Get the currencies held by a smart contract as a map currencyId -> balance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCurrencies(cmd, args, cfg)
		},
	}

	return cmd
}

func runCurrencies(_ *cobra.Command, args []string, cfg *common.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)
	currencies, err := service.GetCurrencies(address)
	if err != nil {
		return err
	}
	if !config.Quiet {
		fmt.Println("Contract currencies:")
	}
	for k, v := range currencies {
		fmt.Printf("%s\t%s", k, v)
		if name := types.GetCurrencyName(k); len(name) > 0 && !config.Quiet {
			fmt.Printf("\t[%s]", name)
		}
		fmt.Println()
	}
	return nil
}
