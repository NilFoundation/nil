package contract

import (
	"bytes"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
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
		gasLimitFlag,
		"Gas limit for read-only call",
	)

	return cmd
}

func runCallReadonly(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1:])
	if err != nil {
		return err
	}

	res, err := service.CallContract(address, params.feeCredit, calldata)
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

	obj, err := abi.Unpack(args[1], hexutil.FromHex(res))
	if err != nil {
		return err
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
