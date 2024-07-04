package receipt

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	libcommon "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("receiptCommand")

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:          "receipt [hash]",
		Short:        "Retrieve a receipt from the cluster",
		Args:         cobra.ExactArgs(1),
		RunE:         runCommand,
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

func runCommand(_ *cobra.Command, args []string) error {
	service := service.NewService(common.GetRpcClient(), nil)

	var hash libcommon.Hash
	if err := hash.Set(args[0]); err != nil {
		return err
	}

	if hash != libcommon.EmptyHash {
		_, err := service.FetchReceiptByHash(params.shardId, hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch receipt")
			return err
		}
	}
	return nil
}
