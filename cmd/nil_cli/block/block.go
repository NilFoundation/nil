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
		Use:   "block [number|hash|tag]",
		Short: "Retrieve a block from the cluster",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint)
		},
		SilenceUsage: true,
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, args []string, rpcEndpoint string) {
	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, nil)

	_, err := service.FetchBlock(params.shardId, args[0])
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch block by number")
	}
}
