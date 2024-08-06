package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func TopUpCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top-up [amount]",
		Short: "Top up wallet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopUp(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runTopUp(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var amount types.Value
	if err := amount.Set(args[0]); err != nil {
		return err
	}

	if _, err := service.GetBalance(cfg.Address); err != nil {
		return err
	}

	if err := service.TopUpViaFaucet(cfg.Address, amount); err != nil {
		return err
	}

	balance, err := service.GetBalance(cfg.Address)
	if err != nil {
		return err
	}

	if !config.Quiet {
		fmt.Print("Wallet balance: ")
	}

	fmt.Println(balance)
	return nil
}
