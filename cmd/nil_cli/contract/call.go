package contract

import (
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCallCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call [address] [calldata or method] [args...]",
		Short: "Call a smart contract",
		Long:  "Call a smart contract with the given address and calldata",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCall(cmd, args, cfg)
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

func prepareArgs(service *service.Service, params *contractParams, args []string) (types.Address, []byte, error) {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return types.EmptyAddress, nil, fmt.Errorf("invalid address: %w", err)
	}

	var calldata []byte
	if strings.HasPrefix(args[1], "0x") && params.abiPath == "" {
		calldata = hexutil.FromHex(args[1])
	} else {
		var err error
		calldata, err = service.ArgsToCalldata(params.abiPath, args[1], args[2:])
		if err != nil {
			return types.EmptyAddress, nil, err
		}
	}
	return address, calldata, nil
}

func runCall(_ *cobra.Command, args []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	address, calldata, err := prepareArgs(service, params, args)
	if err != nil {
		return err
	}

	_, _ = service.CallContract(address, calldata)
	return nil
}
