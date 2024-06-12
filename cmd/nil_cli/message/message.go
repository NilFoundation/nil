package message

import (
	"github.com/NilFoundation/nil/cli/services/message"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("messageCommand")

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "message",
		Short:   "Retrieve a message from the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg.RPCEndpoint)
		},
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.hash,
		hashFlag,
		"",
		"Retrieve message by message hash from the cluster",
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

	client := rpc.NewRPCClient(rpcEndpoint)
	service := message.NewService(client, params.shardId)

	if params.hash != "" {
		_, err := service.FetchMessageByHash(params.hash)
		common.FatalIf(err, logger, "Failed to retrieve message by hash")
		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
