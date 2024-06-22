package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func TopUpCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-up [amount]",
		Short: "Top up wallet",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopUp(cmd, args, cfg)
		},
	}

	return cmd
}

func prepareArgs(args []string) (*types.Uint256, error) {
	var amount types.Uint256
	err := amount.SetFromDecimal(args[0])
	if err != nil {
		return nil, err
	}
	return &amount, err
}

func runTopUp(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	amount, err := prepareArgs(args)
	if err != nil {
		return err
	}

	_, err = service.GetBalance(cfg.Address)
	if err != nil {
		return err
	}

	err = service.TopUpViaFaucet(cfg.Address, amount)
	if err != nil {
		return err
	}

	_, err = service.GetBalance(cfg.Address)
	return err
}
