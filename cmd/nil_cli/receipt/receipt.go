package receipt

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("receiptCommand")

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "receipt",
		Short:   "Retrieve a receipt from the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint)
		},
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().Var(
		&params.hash,
		hashFlag,
		"Retrieve receipt by receipt hash from the cluster",
	)

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string) {
	logger.Info().Msgf("RPC Endpoint: %s", rpcEndpoint)

	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, nil)
	if params.hash != common.EmptyHash {
		_, err := service.FetchReceiptByHash(params.shardId, params.hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch receipt")
		}

		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
