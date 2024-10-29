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
		Short: "Deploy a smart contract",
		Long:  "Deploy the smart contract with the specified hex-bytecode from stdin or from file",
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
		"Specify the shard ID to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"The salt for the deploy message",
	)

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"The path to the ABI file",
	)

	params.amount = types.Value{}
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"The amount of default tokens to send",
	)

	params.currency = types.Value{}
	cmd.Flags().Var(
		&params.currency,
		"currency",
		"The amount of contract currency to generate. This operation cannot be performed when the \"no-wait\" flag is set",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	cmd.Flags().StringVar(
		&params.compileInput,
		compileInput,
		"",
		"The path to the JSON file with the compilation input. Contract will be compiled and deployed on the blockchain and the Cometa service",
	)
}

func runDeploy(_ *cobra.Command, cmdArgs []string, cfg *common.Config) error {
	if !params.currency.IsZero() && params.noWait {
		return errors.New("the \"no-wait\" flag cannot be used with the \"currency\" flag")
	}

	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var cm *cometa.Client
	if len(params.compileInput) != 0 {
		cm = common.GetCometaRpcClient()
	}

	if len(params.compileInput) == 0 && len(cmdArgs) == 0 {
		return errors.New("at least one arg is required (the path to the bytecode file)")
	}

	var bytecode types.Code
	var err error
	var contractData *cometa.ContractData

	if len(params.compileInput) != 0 {
		contractData, err = cm.CompileContract(params.compileInput)
		if err != nil {
			return fmt.Errorf("failed to compile the contract: %w", err)
		}
		var calldata []byte
		if len(cmdArgs) > 0 {
			if params.abiPath == "" {
				return errors.New("The ABI file required for the constructor arguments")
			}
			calldata, err = common.ArgsToCalldata(params.abiPath, "", cmdArgs)
			if err != nil {
				return fmt.Errorf("failed to pack the constructor arguments: %w", err)
			}
		}
		bytecode = append(contractData.InitCode, calldata...) //nolint:gocritic
	} else {
		var filename string
		var args []string
		if argsCount := len(cmdArgs); argsCount > 0 {
			filename = cmdArgs[0]
			args = cmdArgs[1:]
		}

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
			return errors.New("the \"no-wait\" flag cannot be used with contract compilation")
		}
	}
	if receipt == nil || !receipt.AllSuccess() {
		return errors.New("deploy message processing failed")
	}

	if len(params.compileInput) != 0 {
		if err = cm.RegisterContract(contractData, contractAddr); err != nil {
			return fmt.Errorf("failed to register the contract: %w", err)
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
