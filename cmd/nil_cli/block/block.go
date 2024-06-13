package block

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("blockCommand")

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "block",
		Short:   "Retrieve a block from the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint)
		},
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
	cmd.Flags().StringVar(
		&params.number,
		numberFlag,
		"",
		"Retrieve block by block number from the cluster",
	)
	cmd.Flags().StringVar(
		&params.hash,
		hashFlag,
		"",
		"Retrieve block by block hash from the cluster",
	)

	cmd.Flags().Uint32Var(
		(*uint32)(&params.shardId),
		shardIdFlag,
		uint32(types.BaseShardId),
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string) {
	logger.Info().Msgf("RPC Endpoint: %s", rpcEndpoint)

	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, "", params.shardId)

	if params.latest {
		_, err := service.FetchBlockByNumber("latest")
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch latest block")
		}

		return
	}

	if params.number != "" {
		_, err := service.FetchBlockByNumber(params.number)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch block by number")
		}

		return
	}

	if params.hash != "" {
		_, err := service.FetchBlockByHash(params.hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch block by hash")
		}

		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
