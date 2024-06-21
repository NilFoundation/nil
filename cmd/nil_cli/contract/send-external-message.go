package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func GetSendExternalMessageCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-external-message [address] [bytecode or method] [args...]",
		Short: "Send external amessage to the smart contract",
		Long:  "Send an external message to the smart contract with specified bytecode or command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSendExternalMessage(cmd, args, cfg)
		},
		// This command is useful for only rare cases, so it's hidden
		// to avoid confusion for the users between "send" and "send-message"
		Hidden: true,
	}

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	cmd.Flags().BoolVar(
		&params.noSign,
		noSignFlag,
		false,
		"Don't sign external message",
	)

	return cmd
}

func runSendExternalMessage(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	address, calldata, err := prepareArgs(service, params, args)
	if err != nil {
		return err
	}

	_, _ = service.SendExternalMessage(calldata, address, params.noSign)
	return nil
}
