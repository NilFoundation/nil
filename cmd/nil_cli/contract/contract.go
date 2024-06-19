package contract

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("contractCommand")

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "contract",
		Short:   "Interact with contract on the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint, cfg.Address, cfg.PrivateKey)
		},
	}

	setFlags(serverCmd)

	callCmd := GetCallCommand(cfg)
	serverCmd.AddCommand(callCmd)

	sendCmd := GetSendCommand(cfg)
	serverCmd.AddCommand(sendCmd)

	deployCmd := GetDeployCommand(cfg)
	serverCmd.AddCommand(deployCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.code,
		codeFlag,
		"",
		"Get contract code from deployed contract",
	)
	cmd.Flags().Var(
		&params.address,
		addressFlag,
		"Specify the address of the contract to interact with",
	)

	cmd.Flags().Var(
		&params.bytecode,
		bytecodeFlag,
		"Specify the bytecode to be executed with the deployed contract",
	)

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string, address types.Address, privateKey *ecdsa.PrivateKey) {
	logger.Info().Msgf("RPC Endpoint: %s", rpcEndpoint)

	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, privateKey)

	if params.code != "" {
		_, err := service.GetCode(params.code)
		check.PanicIfErr(err)
		return
	}

	if params.address != types.EmptyAddress && params.bytecode != nil {
		_, err := service.RunContract(address, params.bytecode, params.address)
		check.PanicIfErr(err)
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
