package block

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("blockCommand")

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "block",
		Short:   "Retrieve a block from the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint)
		},
		SilenceUsage: true,
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&params.latest,
		latestFlag,
		false,
		"Retrieve the latest block from the cluster",
	)
	cmd.Flags().Var(
		&params.blockNrOrHash,
		numberFlag,
		"Retrieve block by block number from the cluster",
	)
	cmd.Flags().Var(
		&params.blockNrOrHash,
		hashFlag,
		"Retrieve block by block hash from the cluster",
	)
	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string) {
	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, nil)

	if params.latest {
		_, err := service.FetchBlock(params.shardId, "latest")
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch latest block")
		}

		return
	}

	if params.blockNrOrHash.IsValid() {
		_, err := service.FetchBlock(params.shardId, params.blockNrOrHash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch block by number")
		}

		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	if cmd.Flag(hashFlag).Changed && cmd.Flag(numberFlag).Changed {
		return errMultipleSelected
	}

	return params.initRawParams()
}
