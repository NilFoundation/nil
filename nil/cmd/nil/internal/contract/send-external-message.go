package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetSendExternalMessageCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-external-message [address] [bytecode or method] [args...]",
		Short: "Send an external message to a smart contract",
		Long:  "Send an external message to the smart contract with the specified bytecode or command",
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
		"The path to the ABI file",
	)

	cmd.Flags().BoolVar(
		&params.noSign,
		noSignFlag,
		false,
		"Define whether the external message should be signed",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	return cmd
}

func runSendExternalMessage(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1], args[2:])
	if err != nil {
		return err
	}

	msgHash, err := service.SendExternalMessage(calldata, address, params.noSign)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}
	fmt.Println(msgHash)
	return nil
}
