package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
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

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	_, _ = service.GetCurrencies(address)
	return nil
}
