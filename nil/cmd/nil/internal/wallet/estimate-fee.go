package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetEstimateFeeCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "estimate-fee [address] [calldata or method] [args...]",
		Short: "Get the recommended fees (internal and external) for a message sent by the wallet",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEstimateFee(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&params.AbiPath, abiFlag, "", "The path to the ABI file")
	cmd.Flags().Var(&params.value, valueFlag, "The value for the transfer")
	cmd.Flags().BoolVar(&params.deploy, deployFlag, false, "Set the \"deploy\" flag")

	return cmd
}

func runEstimateFee(cmd *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	abi, err := common.ReadAbiFromFile(params.AbiPath)
	if err != nil {
		return err
	}

	calldata, err := common.PrepareArgs(abi, args[1], args[2:])
	if err != nil {
		return err
	}

	kind := types.ExecutionMessageKind
	if params.deploy {
		kind = types.DeployMessageKind
	}

	intMsg := &types.InternalMessagePayload{
		Data:        calldata,
		To:          address,
		ForwardKind: types.ForwardKindRemaining,
		Kind:        kind,
		Value:       params.value,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return err
	}

	walletCalldata, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
	if err != nil {
		return err
	}

	value, err := service.EstimateFee(cfg.Address, walletCalldata, types.MessageFlags{}, types.Value{})
	if err != nil {
		return err
	}

	if !common.Quiet {
		fmt.Print("Estimated fee: ")
	}
	fmt.Println(value)

	return nil
}
