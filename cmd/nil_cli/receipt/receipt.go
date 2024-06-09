package receipt

import (
	"github.com/NilFoundation/nil/cli/services/receipt"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("receiptCommand")

func GetCommand(rpcEndpoint string) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "receipt",
		Short:   "Retrieve a receipt from the cluster",
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
		"Retrieve receipt by receipt hash from the cluster",
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
	service := receipt.NewService(client, params.shardId)
	if params.hash != "" {
		_, err := service.FetchReceiptByHash(params.hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch receipt")
		}

		return
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
