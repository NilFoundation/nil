package message

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	libcommon "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("messageCommand")

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "message [hash]",
		Short: "Retrieve a message from the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, args, cfg.RPCEndpoint)
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

func runCommand(_ *cobra.Command, args []string, rpcEndpoint string) error {
	client := rpc.NewClient(rpcEndpoint)
	service := service.NewService(client, nil)

	var hash libcommon.Hash
	if err := hash.Set(args[0]); err != nil {
		return err
	}

	if hash != libcommon.EmptyHash {
		if _, err := service.FetchMessageByHash(params.shardId, hash); err != nil {
			logger.Error().Err(err).Msg("Failed to fetch message")
			return err
		}
	}
	return nil
}
