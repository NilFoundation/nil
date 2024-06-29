package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/spf13/cobra"
)

func InfoCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Returns wallet's address and public key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return infoBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func infoBalance(_ *cobra.Command, _ []string, cfg *common.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	if _, _, err := service.GetInfo(cfg.Address); err != nil {
		return err
	}
	return nil
}
