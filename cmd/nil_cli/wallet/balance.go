package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func BalanceCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Returns wallet's balance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, args, cfg)
		},
	}

	return cmd
}

func runBalance(_ *cobra.Command, _ []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	_, _ = service.GetBalance(cfg.Address)
	return nil
}
