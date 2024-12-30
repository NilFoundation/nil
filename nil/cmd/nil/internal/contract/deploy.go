package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	libcommon "github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetDeployCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy a smart contract",
		Long:  "Deploy a smart contract with the specified hex-bytecode from stdin or from a file",
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
		"Specify the shard ID to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"The salt for deployment message",
	)

	cmd.Flags().StringVar(
		&params.AbiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	cmd.Flags().Var(
		&params.FeeCredit,
		feeCreditFlag,
		"The deployment fee credit. If  set to 0, it will be estimated automatically",
	)
}

func runDeploy(cmd *cobra.Command, cmdArgs []string, cfg *common.Config) error {
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

	payload := types.BuildDeployPayload(bytecode, libcommon.Hash(params.salt.Bytes32()))

	msgHash, addr, err := service.DeployContractExternal(params.shardId, payload, params.FeeCredit)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(msgHash); err != nil {
			return err
		}
	}

	if !common.Quiet {
		fmt.Print("Message hash: ")
	}
	fmt.Println(msgHash)

	if !common.Quiet {
		fmt.Print("Contract address: ")
	}
	fmt.Println(addr)

	return nil
}
