package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func BalanceCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Get the balance of the wallet whose address specified in config.address field",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runBalance(_ *cobra.Command, _ []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)
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
