package wallet

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	libcommon "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func DeployCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy smart contract",
		Long:  "Deploy smart contract with specified hex-bytecode from stdin or from file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, args, cfg)
		},
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

	params.amount = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"Amount of tokens to send",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)
}

func runDeploy(_ *cobra.Command, cmdArgs []string, cfg *config.Config) error {
	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	var filename string
	var args []string
	if argsCount := len(cmdArgs); argsCount > 0 {
		filename = cmdArgs[0]
		args = cmdArgs[1:]
	}

	bytecode, err := common.ReadBytecode(filename, params.abiPath, args)
	if err != nil {
		return err
	}

	bytecode = types.BuildDeployPayload(bytecode, libcommon.Hash(params.salt.Bytes32()))

	msgHash, _, err := service.DeployContractViaWallet(params.shardId, cfg.Address, bytecode, &params.amount)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	return nil
}
