package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetBalanceCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance [address]",
		Short: "Get the balance of a smart contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runBalance(cmd *cobra.Command, args []string, cfg *common.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)
	balance, err := service.GetBalance(address)
	if err != nil {
		return err
	}
	if !common.Quiet {
		fmt.Print("Contract balance: ")
	}
	fmt.Println(balance)
	return nil
}
