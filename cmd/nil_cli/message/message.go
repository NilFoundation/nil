package message

import (
	"github.com/NilFoundation/nil/cli/services/message"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common"
	"github.com/spf13/cobra"
)

var logger = common.NewLogger("messageCommand")

func GetCommand(rpcEndpoint string) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "message",
		Short:   "Retrieve a message from the cluster",
		PreRunE: runPreRun,
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, rpcEndpoint)
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
		0,
		"Specify the shard id to interact with",
	)
}

func runCommand(_ *cobra.Command, _ []string, rpcEndpoint string) {
	logger.Info().Msgf("RPC Endpoint: %s", rpcEndpoint)

	client := rpc.NewRPCClient(rpcEndpoint)
	service := message.NewService(client, params.shardId)

	if params.hash != "" {
		_, err := service.FetchMessageByHash(params.hash)
		if err != nil {
			logger.Fatal().Msgf("Failed to retrieve message by hash")
		}

		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
