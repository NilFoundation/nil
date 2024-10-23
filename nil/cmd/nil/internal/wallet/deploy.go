package wallet

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/spf13/cobra"
)

func DeployCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy smart contract",
		Long:  "Deploy smart contract with specified hex-bytecode from stdin or from file",
		Args:  cobra.ArbitraryArgs,
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

	cmd.Flags().StringVar(
		&params.compileInput,
		compileInput,
		"",
		"JSON file with compilation input. Contract will be compiled and deployed on blockchain and cometa",
	)
}

func runDeploy(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
	if !params.currency.IsZero() && params.noWait {
		return errors.New("no-wait flag can't be used with currency flag")
	}

	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var cm *cometa.Client
	if len(params.compileInput) != 0 {
		cm = common.GetCometaRpcClient()
	}

	if len(params.compileInput) == 0 && len(cmdArgs) == 0 {
		return errors.New("at least one arg required(path to bytecode file)")
	}

	var filename string
	var args []string
	if argsCount := len(cmdArgs); argsCount > 0 {
		filename = cmdArgs[0]
		args = cmdArgs[1:]
	}

	var bytecode types.Code
	var err error
	var contractData *cometa.ContractData

	if len(params.compileInput) != 0 {
		contractData, err = cm.CompileContract(params.compileInput)
		if err != nil {
			return fmt.Errorf("failed to compile contract: %w", err)
		}
		bytecode = contractData.InitCode
	} else {
		bytecode, err = common.ReadBytecode(filename, params.abiPath, args)
		if err != nil {
			return err
		}
	}

	payload := types.BuildDeployPayload(bytecode, params.salt.Bytes32())

	msgHash, contractAddr, err := service.DeployContractViaWallet(params.shardId, cfg.Address, payload, params.amount)
	if err != nil {
		return err
	}

	var receipt *jsonrpc.RPCReceipt
	if !params.noWait {
		if receipt, err = service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	} else {
		if len(params.compileInput) != 0 {
			return errors.New("no-wait flag can't be used with contract compilation")
		}
	}
	if receipt == nil || !receipt.AllSuccess() {
		return errors.New("deploy message processing failed")
	}

	if len(params.compileInput) != 0 {
		if err = cm.RegisterContract(contractData, contractAddr); err != nil {
			return fmt.Errorf("failed to register contract: %w", err)
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
