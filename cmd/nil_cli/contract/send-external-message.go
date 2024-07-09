package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetSendExternalMessageCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-external-message [address] [bytecode or method] [args...]",
		Short: "Send external amessage to the smart contract",
		Long:  "Send an external message to the smart contract with specified bytecode or command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSendExternalMessage(cmd, args, cfg)
		},
		SilenceUsage: true,
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

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)

	return cmd
}

func runSendExternalMessage(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1:])
	if err != nil {
		return err
	}

	msgHash, err := service.SendExternalMessage(calldata, address, params.noSign)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}
	fmt.Println(msgHash)
	return nil
}
