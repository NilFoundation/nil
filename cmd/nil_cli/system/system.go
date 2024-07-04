package system

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	var svc *service.Service

	configCmd := &cobra.Command{
		Use:          "system",
		Short:        "Request system-wide information",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Parent().Parent().PersistentPreRunE(cmd, args); err != nil {
				return err
			}
			svc = service.NewService(common.GetRpcClient(), cfg.PrivateKey)
			return nil
		},
	}

	shardsCmd := &cobra.Command{
		Use:          "shards",
		Short:        "Print list of shards",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)
			if _, err := svc.GetShards(); err != nil {
				return err
			}
			return nil
		},
	}

	gasPriceCmd := &cobra.Command{
		Use:          "gas-price [shard-id]",
		Short:        "Returns gas price for specific shard",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)

			var shardId types.ShardId
			if err := shardId.Set(args[0]); err != nil {
				return err
			}

			if _, err := svc.GetGasPrice(shardId); err != nil {
				return err
			}
			return nil
		},
	}

	chainIdCmd := &cobra.Command{
		Use:          "chain-id",
		Short:        "Returns chainId of current network",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)
			if _, err := svc.GetChainId(); err != nil {
				return err
			}
			return nil
		},
	}

	configCmd.AddCommand(shardsCmd)
	configCmd.AddCommand(gasPriceCmd)
	configCmd.AddCommand(chainIdCmd)

	return configCmd
}
