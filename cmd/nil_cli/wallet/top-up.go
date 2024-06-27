package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func TopUpCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-up [amount]",
		Short: "Top up wallet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopUp(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runTopUp(_ *cobra.Command, args []string, cfg *common.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	var amount types.Uint256
	if err := amount.SetFromDecimal(args[0]); err != nil {
		return err
	}

	if _, err := service.GetBalance(cfg.Address); err != nil {
		return err
	}

	if err := service.TopUpViaFaucet(cfg.Address, &amount); err != nil {
		return err
	}

	_, err := service.GetBalance(cfg.Address)
	return err
}
