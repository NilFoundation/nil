package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCallReadonlyCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-readonly [address] [calldata or method] [args...]",
		Short: "Readonly call of a smart contract",
		Long:  "Readonly call a smart contract with the given address and calldata",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCallReadonly(cmd, args, cfg)
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

func runCallReadonly(_ *cobra.Command, args []string, cfg *config.Config) error {
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

	_, _ = service.CallContract(address, calldata)
	return nil
}
