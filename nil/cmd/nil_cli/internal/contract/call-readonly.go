package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"
)

func GetCallReadonlyCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-readonly [address] [calldata or method] [args...]",
		Short: "Readonly call of a smart contract",
		Long:  "Readonly call a smart contract with the given address and calldata",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCallReadonly(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	params.feeCredit = types.GasToValue(100_000)
	cmd.Flags().Var(
		&params.feeCredit,
		feeCreditFlag,
		"Fee credit for read-only call",
	)

	cmd.Flags().StringVar(
		&params.inOverridesPath,
		inOverridesFlag,
		"",
		"Input state overrides",
	)

	cmd.Flags().StringVar(
		&params.outOverridesPath,
		outOverridesFlag,
		"",
		"Output state overrides",
	)

	return cmd
}

func runCallReadonly(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	var inOverrides *jsonrpc.StateOverrides
	if params.inOverridesPath != "" {
		inOverridesData, err := os.ReadFile(params.inOverridesPath)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(inOverridesData, &inOverrides); err != nil {
			return err
		}
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1:])
	if err != nil {
		return err
	}

	res, err := service.CallContract(address, params.feeCredit, calldata, inOverrides)
	if err != nil {
		return err
	}

	abiFile, err := os.ReadFile(params.abiPath)
	if err != nil {
		return err
	}

	abi, err := abi.JSON(bytes.NewReader(abiFile))
	if err != nil {
		return err
	}

	obj, err := abi.Unpack(args[1], res.Data)
	if err != nil {
		return err
	}

	if params.outOverridesPath != "" {
		outOverridesData, err := json.Marshal(res.StateOverrides)
		if err != nil {
			return err
		}

		if err := os.WriteFile(params.outOverridesPath, outOverridesData, 0o600); err != nil {
			return err
		}
	}

	if len(abi.Methods[args[1]].Outputs) == 0 {
		fmt.Println("Success, no result")
		return nil
	}

	if !config.Quiet {
		fmt.Println("Success, result:")
	}
	for i, output := range abi.Methods[args[1]].Outputs {
		fmt.Printf("%s: %v\n", output.Type, obj[i])
	}

	return nil
}
