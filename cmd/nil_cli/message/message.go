package message

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	libcommon "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "message",
		Short:   "Retrieve a message from the cluster",
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
	cmd.Flags().Var(
		&params.hash,
		hashFlag,
		"Retrieve message by message hash from the cluster",
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

	if params.hash != libcommon.EmptyHash {
		_, err := service.FetchMessageByHash(params.shardId, params.hash)
		check.PanicIfErr(err)
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error { return params.initRawParams() }
