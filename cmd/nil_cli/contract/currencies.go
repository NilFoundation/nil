package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCurrenciesCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "currencies [address]",
		Short: "Returns a smart contract currencies balance as a map currencyId -> balance",
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

	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)
	currencies, _ := service.GetCurrencies(address)
	if !config.Quiet {
		fmt.Print("Contract currencies: ")
	}
	for k, v := range currencies {
		fmt.Printf("%v\t%v\n", k, v)
	}
	return nil
}
