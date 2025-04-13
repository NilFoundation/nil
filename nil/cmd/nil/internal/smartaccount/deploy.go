package smartaccount

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/spf13/cobra"
)

func DeployCommand(cfg *common.Config) *cobra.Command {
	params := &smartAccountParams{
		Params: &common.Params{},
	}

	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy a smart contract",
		Long:  "Deploy the smart contract with the specified hex-bytecode from stdin or from file",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, args, cfg, params)
		},
		SilenceUsage: true,
	}

	setDeployFlags(cmd, params)

	return cmd
}

func setDeployFlags(cmd *cobra.Command, params *smartAccountParams) {
	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard ID to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"The salt for the deploy transaction",
	)

	cmd.Flags().StringVar(
		&params.AbiPath,
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

	params.token = types.Value{}
	cmd.Flags().Var(
		&params.token,
		"token",
		"The amount of contract token to generate. This operation cannot be performed when the \"no-wait\" flag is set",
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
		"The path to the JSON file with the compilation input."+
			" Contract will be compiled and deployed on the blockchain and the Cometa service",
	)
}

func runDeploy(cmd *cobra.Command, cmdArgs []string, cfg *common.Config, params *smartAccountParams) error {
	if !params.token.IsZero() && params.noWait {
		return errors.New("the \"no-wait\" flag cannot be used with the \"token\" flag")
	}

	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

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
			abi, err := common.ReadAbiFromFile(params.AbiPath)
			if err != nil {
				return err
			}
			calldata, err = common.ArgsToCalldata(abi, "", cmdArgs)
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

		bytecode, err = common.ReadBytecode(filename, params.AbiPath, args)
		if err != nil {
			return err
		}
	}

	payload := types.BuildDeployPayload(bytecode, params.salt.Bytes32())

	txnHash, contractAddr, err := service.DeployContractViaSmartAccount(
		params.shardId, cfg.Address, payload, params.amount)
	if err != nil {
		if errors.Is(err, rpc.ErrTxnDataTooLong) {
			return fmt.Errorf(
				`failed to marshal transaction: %w.
It appears that your code exceeds the maximum supported size.
Try compiling your contract with the usage of solc --optimize flag,
providing small values to --optimize-runs.
For more information go to
https://ethereum.org/en/developers/tutorials/downsizing-contracts-to-fight-the-contract-size-limit/`, err)
		}
		return err
	}

	var receipt *jsonrpc.RPCReceipt
	if !params.noWait {
		if receipt, err = service.WaitForReceipt(txnHash); err != nil {
			return err
		}
	} else {
		if len(params.compileInput) != 0 {
			return errors.New("the \"no-wait\" flag cannot be used with contract compilation")
		}
	}
	if receipt == nil || !receipt.AllSuccess() {
		return errors.New("deploy transaction processing failed")
	}

	if len(params.compileInput) != 0 {
		if err = cm.RegisterContractData(contractData, contractAddr); err != nil {
			return fmt.Errorf("failed to register the contract: %w", err)
		}
	}

	if !common.Quiet {
		fmt.Print("Transaction hash: ")
	}
	fmt.Printf("0x%x\n", txnHash)

	if !common.Quiet {
		fmt.Print("Contract address: ")
	}
	fmt.Printf("0x%x\n", contractAddr)
	return nil
}
