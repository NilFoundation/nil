package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetEstimateFeeCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "estimate-fee [address] [calldata or method] [args...]",
		Short: "Returns recommended fee for the message",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runEstimateFee(args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&params.abiPath, abiFlag, "", "Path to ABI file")
	cmd.Flags().Var(&params.value, valueFlag, "Value for transfer")
	cmd.Flags().BoolVar(&params.internal, internalFlag, false, "Set \"internal\" flag")
	cmd.Flags().BoolVar(&params.deploy, deployFlag, false, "Set \"deploy\" flag")

	return cmd
}

func runEstimateFee(args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1], args[2:])
	if err != nil {
		return err
	}

	var msgFlags types.MessageFlags
	if params.internal {
		msgFlags.SetBit(types.MessageFlagInternal)
	}
	if params.deploy {
		msgFlags.SetBit(types.MessageFlagDeploy)
	}

	res, err := service.EstimateFee(address, calldata, msgFlags, params.value)
	if err != nil {
		return err
	}

	if !config.Quiet {
		fmt.Print("Estimated fee: ")
	}
	fmt.Println(res)

	return nil
}
