package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetAddressCommand(cfg *common.Config) *cobra.Command {
	params := &contractParams{
		Params: &common.Params{},
	}

	cmd := &cobra.Command{
		Use:   "address [path to file] [args...]",
		Short: "Calculate the address of a smart contract",
		Long:  "Calculate the address of a smart contract by the specified hex-bytecode from stdin or from a file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddress(cmd, args, cfg, params)
		},
		SilenceUsage: true,
	}

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard ID to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"The salt for the deployment transaction",
	)

	cmd.Flags().StringVar(
		&params.AbiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	return cmd
}

func runAddress(cmd *cobra.Command, cmdArgs []string, cfg *common.Config, params *contractParams) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

	var filename string
	var args []string
	if len(cmdArgs) > 0 {
		filename = cmdArgs[0]
		args = cmdArgs[1:]
	}

	bytecode, err := common.ReadBytecode(filename, params.AbiPath, args)
	if err != nil {
		return err
	}

	address := service.ContractAddress(params.shardId, params.salt, bytecode)
	if !common.Quiet {
		fmt.Print("Contract address: ")
	}
	fmt.Println(address.Hex())

	return nil
}
