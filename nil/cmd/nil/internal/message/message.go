package message

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	libcommon "github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("messageCommand")

func GetCommand(cfgPath *string) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "message [hash]",
		Short: "Retrieve a message from the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCommand(cfgPath, args)
		},
		SilenceUsage: true,
	}

	setFlags(serverCmd)

	serverCmd.AddCommand(GetInternalMessageCommand())

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runCommand(cfgPath *string, args []string) error {
	cfg, err := common.LoadConfig(*cfgPath, logger)
	if err != nil {
		return err
	}
	common.InitRpcClient(cfg, logger)

	service := cliservice.NewService(common.GetRpcClient(), nil)

	var hash libcommon.Hash
	if err := hash.Set(args[0]); err != nil {
		return err
	}

	if hash != libcommon.EmptyHash {
		msgDataJson, err := service.FetchMessageByHash(params.shardId, hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch message")
			return err
		}
		if !config.Quiet {
			fmt.Print("Message data: ")
		}
		fmt.Println(string(msgDataJson))
	}
	return nil
}
