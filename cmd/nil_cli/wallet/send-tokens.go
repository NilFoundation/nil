package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func SendTokensCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-tokens [address] [amount]",
		Short: "Transfer tokens to specific address",
		Long:  "Transfer some amount of tokens to specific address",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTransfer(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)

	params.gasLimit = *types.NewUint256(100_000)
	cmd.Flags().Var(
		&params.gasLimit,
		gasLimitFlag,
		"Gas limit",
	)

	return cmd
}

func runTransfer(_ *cobra.Command, args []string, cfg *common.Config) error {
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

	msgHash, err := service.RunContract(cfg.Address, nil, &params.gasLimit, amount, address)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	return nil
}
