package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func GetCallCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call [address] [calldata]",
		Short: "Call a smart contract",
		Long:  "Call a smart contract with the given address and calldata",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCall(cmd, args, cfg)
		},
	}
	return cmd
}

func runCall(_ *cobra.Command, args []string, cfg *config.Config) error {
	if err := callParams.address.Set(args[0]); err != nil {
		return fmt.Errorf("Invalid address: %w", err)
	}

	callParams.code = args[1]

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, nil)
	_, _ = service.CallContract(callParams.address, callParams.code)

	return nil
}
