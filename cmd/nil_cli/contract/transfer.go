package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetTransferCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer [address] [amount]",
		Short: "Transfer coins to specific address",
		Long:  "Transfer some amount of coins to specific address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransfer(cmd, args, cfg)
		},
	}

	return cmd
}

func runTransfer(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	amount := types.NewUint256(0)
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	_, _ = service.RunContract(cfg.Address, nil, amount, address)
	return nil
}
