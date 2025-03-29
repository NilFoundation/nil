package block

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
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
		"Specify the shard ID to interact with",
	)

	cmd.Flags().BoolVar(&params.jsonOutput, jsonFlag, false, "Enable JSON output")
	cmd.Flags().BoolVar(&params.fullOutput, fullFlag, false, "Do not cut any data")
	cmd.Flags().BoolVar(&params.noColor, noColorFlag, false, "Do not colorize the output")
}

func runCommand(cmd *cobra.Command, args []string) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), nil, nil)

	blockData, err := service.FetchDebugBlock(
		params.shardId, args[0], params.jsonOutput, params.fullOutput, params.noColor)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch the block by number")
		return err
	}
	if blockData != nil {
		fmt.Println(string(blockData))
	} else {
		fmt.Println("null")
	}

	return nil
}
