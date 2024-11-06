package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/spf13/cobra"
)

func GetCallReadonlyCommand(cfg *common.Config) *cobra.Command {
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

	calldata, err := common.PrepareArgs(params.abiPath, args[1], args[2:])
	if err != nil {
		return err
	}

	handler := func(res *jsonrpc.CallRes) ([]*common.ArgValue, error) {
		if res.Error != "" {
			return nil, fmt.Errorf("error during the call: %s", res.Error)
		}

		return common.CalldataToArgs(params.abiPath, args[1], res.Data)
	}

	return common.CallReadonly(
		service, address, calldata, params.feeCredit, handler,
		params.inOverridesPath, params.outOverridesPath,
		params.withDetails, config.Quiet,
	)
}
