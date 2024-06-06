package contract

import (
	"github.com/NilFoundation/nil/cli/services/contract"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = common.NewLogger("contractCommand")

func GetCommand(rpcEndpoint string, privateKey string) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "contract",
		Short:   "Interact with contract on the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, rpcEndpoint, privateKey)
		},
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.deploy,
		deployFlag,
		"",
		"Deploy new contract by specifying deployment bytecode",
	)
	cmd.Flags().StringVar(
		&params.code,
		codeFlag,
		"",
		"Get contract code from deployed contract",
	)
	cmd.Flags().StringVar(
		&params.address,
		addressFlag,
		"",
		"Specify the address of the contract to interact with",
	)

	cmd.Flags().StringVar(
		&params.bytecode,
		bytecodeFlag,
		"",
		"Specify the bytecode to be executed with the deployed contract",
	)

	cmd.Flags().Uint32Var(
		(*uint32)(&params.shardId),
		shardIdFlag,
		uint32(types.BaseShardId),
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string, privateKey string) {
	logger.Info().Msgf("RPC Endpoint: %s", rpcEndpoint)

	client := rpc.NewRPCClient(rpcEndpoint)
	service := contract.NewService(client, privateKey, params.shardId)
	if params.deploy != "" {
		_, err := service.DeployContract(params.deploy)
		if err != nil {
			logger.Fatal().Msgf("Failed to deploy contract")
		}

		return
	}

	if params.code != "" {
		_, err := service.GetCode(params.code)
		if err != nil {
			logger.Fatal().Msgf("Failed to get contract code")
		}

		return
	}

	if params.address != "" && params.bytecode != "" {
		_, err := service.RunContract(params.bytecode, params.address)
		if err != nil {
			logger.Fatal().Msgf("Failed to run contract")
		}
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
