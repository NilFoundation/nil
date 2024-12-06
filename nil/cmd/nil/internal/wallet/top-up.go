package wallet

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/spf13/cobra"
)

func TopUpCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-up [amount] [currency-id]",
		Short: "Top up the wallet specified in the config file",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopUp(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runTopUp(_ *cobra.Command, args []string, cfg *common.Config) error {
	var amount types.Value
	if err := amount.Set(args[0]); err != nil {
		return err
	}

	var currId string
	if len(args) > 1 {
		currId = args[1]
	}

	return common.RunTopUp("wallet", cfg, cfg.Address, amount, currId, common.Quiet)
}
