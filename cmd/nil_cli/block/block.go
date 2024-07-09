package block

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("blockCommand")

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:          "block [number|hash|tag]",
		Short:        "Retrieve a block from the cluster",
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

	blockDataJSON, err := service.FetchBlock(params.shardId, args[0])
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch block by number")
		return err
	}
	if !config.Quiet {
		fmt.Print("Block data: ")
	}
	fmt.Println(string(blockDataJSON))
	return nil
}
