package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func GetSendCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [address] [bytecode or method] [args...]",
		Short: "Send amessage to the smart contract",
		Long:  "Send a message to the smart contract with specified bytecode or command",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSend(cmd, args, cfg)
		},
	}

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	return cmd
}

func runSend(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	address, calldata, err := prepareArgs(service, params, args)
	if err != nil {
		return err
	}

	_, _ = service.RunContract(cfg.Address, calldata, address)
	return nil
}
