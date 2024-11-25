package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/spf13/cobra"
)

func CallReadonlyCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-readonly [address] [calldata or method] [args...]",
		Short: "Perform a read-only call to a smart contract",
		Long:  "Perform a read-only call to the smart contract with the given address and calldata",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCallReadonly(args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	params.feeCredit = types.GasToValue(100_000)
	cmd.Flags().Var(
		&params.feeCredit,
		feeCreditFlag,
		"The fee credit for the read-only call",
	)

	cmd.Flags().StringVar(
		&params.inOverridesPath,
		inOverridesFlag,
		"",
		"The input state overrides",
	)

	cmd.Flags().StringVar(
		&params.outOverridesPath,
		outOverridesFlag,
		"",
		"The output state overrides",
	)

	cmd.Flags().BoolVar(
		&params.withDetails,
		withDetailsFlag,
		false,
		"Define whether to show the tokens used and outbound messages",
	)

	return cmd
}

func runCallReadonly(args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	contractCalldata, err := common.PrepareArgs(params.abiPath, args[1], args[2:])
	if err != nil {
		return err
	}

	intMsg := &types.InternalMessagePayload{
		Data:        contractCalldata,
		To:          address,
		FeeCredit:   params.feeCredit,
		ForwardKind: types.ForwardKindNone,
		Kind:        types.ExecutionMessageKind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return err
	}

	walletCalldata, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
	if err != nil {
		return err
	}

	handler := func(res *jsonrpc.CallRes) ([]*common.ArgValue, []*common.NamedArgValues, error) {
		if res.Error != "" {
			return nil, nil, fmt.Errorf("error during sending the message to the wallet: %s", res.Error)
		}

		if outMsgLen := len(res.OutMessages); outMsgLen != 1 {
			return nil, nil, fmt.Errorf("expected one outbound message but got %d", outMsgLen)
		}

		if outMsgErr := res.OutMessages[0].Error; outMsgErr != "" {
			return nil, nil, fmt.Errorf("error during processing the wallet message: %s", outMsgErr)
		}

		abi, err := common.ReadAbiFromFile(params.abiPath)
		if err != nil {
			return nil, nil, err
		}

		logs, err := common.DecodeLogs(abi, res.OutMessages[0].Logs)
		if err != nil {
			return nil, nil, err
		}

		result, err := common.CalldataToArgs(abi, args[1], res.OutMessages[0].Data)
		if err != nil {
			return nil, nil, err
		}
		return result, logs, nil
	}

	return common.CallReadonly(
		service, cfg.Address, walletCalldata, params.feeCredit, handler,
		params.inOverridesPath, params.outOverridesPath,
		params.withDetails, config.Quiet,
	)
}
