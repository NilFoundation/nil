package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetBalanceCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Returns a smart contract balance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runBalance(_ *cobra.Command, args []string, cfg *common.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)
	balance, err := service.GetBalance(address)
	if err != nil {
		return err
	}
	if !config.Quiet {
		fmt.Print("Contract balance: ")
	}
	fmt.Println(balance)
	return nil
}
