package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetAddressCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address [path to file] [args...]",
		Short: "Calculate the address of a smart contract",
		Long:  "Calculate the address of a smart contract by the specified hex-bytecode from stdin or from a file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddress(cmd, args, cfg)
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
		"The salt for the deployment message",
	)

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	return cmd
}

func runAddress(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)

	var filename string
	var args []string
	if len(cmdArgs) > 0 {
		filename = cmdArgs[0]
		args = cmdArgs[1:]
	}

	bytecode, err := common.ReadBytecode(filename, params.abiPath, args)
	if err != nil {
		return err
	}

	address := service.ContractAddress(params.shardId, params.salt, bytecode)
	if !config.Quiet {
		fmt.Print("Contract address: ")
	}
	fmt.Println(address.Hex())

	return nil
}
