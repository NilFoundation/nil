package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetBalanceCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Returns a smart contract balance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runBalance(_ *cobra.Command, args []string, cfg *common.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	_, _ = service.GetBalance(address)
	return nil
}
