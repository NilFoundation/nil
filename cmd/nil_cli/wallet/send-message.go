package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func SendMessageCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-message [address] [bytecode or method] [args...]",
		Short: "Send a message to the smart contract via the wallet",
		Long:  "Send a message to the smart contract with specified bytecode or command via the wallet",
		Args:  cobra.MinimumNArgs(1),
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

	params.amount = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"Amount of tokens to send",
	)

	return cmd
}

func runSend(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1:])
	if err != nil {
		return err
	}

	_, _ = service.RunContract(cfg.Address, calldata, &params.amount, address)
	return nil
}
