package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("addressCommand")

func GetAddressCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address [path to file] [args...]",
		Short: "Calculate smart contract address",
		Long:  "Calculate smart contract address by specified hex-bytecode from stdin or from file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddress(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"Salt for deploy message",
	)

	cmd.Flags().BoolVarP(
		&params.quiet,
		quietFlag,
		"q",
		false,
		"Quiet mode (print only the contract address and exit)",
	)

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	return cmd
}

func runAddress(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

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
	if !params.quiet {
		logger.Info().Msgf("Contract address: %s", address)
	} else {
		fmt.Println(address)
	}

	return nil
}
