package system

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	var svc *cliservice.Service

	configCmd := &cobra.Command{
		Use:          "system",
		Short:        "Request system-wide information",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Parent().Parent().PersistentPreRunE(cmd, args); err != nil {
				return err
			}
			svc = cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)
			return nil
		},
	}

	shardsCmd := &cobra.Command{
		Use:          "shards",
		Short:        "Print a list of shards",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)
			list, err := svc.GetShards()
			if err != nil {
				return err
			}
			if !common.Quiet {
				fmt.Println("Shards: ")
			}
			fmt.Print(cliservice.ShardsToString(list))
			return nil
		},
	}

	gasPriceCmd := &cobra.Command{
		Use:          "gas-price [shard-id]",
		Short:        "Get the gas price for a specific shard",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)

			var shardId types.ShardId
			if err := shardId.Set(args[0]); err != nil {
				return err
			}

			val, err := svc.GetGasPrice(shardId)
			if err != nil {
				return err
			}
			if !common.Quiet {
				fmt.Printf("Gas price for shard %v: ", shardId)
			}
			fmt.Println(val)
			return nil
		},
	}

	chainIdCmd := &cobra.Command{
		Use:          "chain-id",
		Short:        "Returns the chain ID of the current network",
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check.PanicIfNot(svc != nil)
			chainId, err := svc.GetChainId()
			if err != nil {
				return err
			}
			if !common.Quiet {
				fmt.Print("ChainId: ")
			}
			fmt.Println(chainId)
			return nil
		},
	}

	configCmd.AddCommand(shardsCmd)
	configCmd.AddCommand(gasPriceCmd)
	configCmd.AddCommand(chainIdCmd)

	return configCmd
}
