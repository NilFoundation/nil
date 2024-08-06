package wallet

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func DeployCommand(cfg *common.Config) *cobra.Command {
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

	params.amount = types.Value{}
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"Amount of tokens to send",
	)

	params.currency = types.Value{}
	cmd.Flags().Var(
		&params.currency,
		"currency",
		"Amount of contract currency to generate. You can't perform this operation with no-wait flag.",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)
}

func runDeploy(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
	if !params.currency.IsZero() && params.noWait {
		return errors.New("no-wait flag can't be used with currency flag")
	}

	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

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

	payload := types.BuildDeployPayload(bytecode, params.salt.Bytes32())

	msgHash, contractAddr, err := service.DeployContractViaWallet(params.shardId, cfg.Address, payload, params.amount)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}
	fmt.Printf("0x%x\n", msgHash)

	if !config.Quiet {
		fmt.Print("Contract address: ")
	}
	fmt.Printf("0x%x\n", contractAddr)
	return nil
}
