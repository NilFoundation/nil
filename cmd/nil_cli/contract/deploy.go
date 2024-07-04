package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	libcommon "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetDeployCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy smart contract",
		Long:  "Deploy smart contract with specified hex-bytecode from stdin or from file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	setDeployFlags(cmd)

	return cmd
}

func setDeployFlags(cmd *cobra.Command) {
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

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)
}

func runDeploy(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
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

	payload := types.BuildDeployPayload(bytecode, libcommon.Hash(params.salt.Bytes32()))

	msgHash, addr, err := service.DeployContractExternal(params.shardId, payload)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(addr.ShardId(), msgHash); err != nil {
			return err
		}
	}

	return nil
}
