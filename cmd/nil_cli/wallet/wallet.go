package wallet

import (
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "wallet",
		Short: "Interact with wallet on the cluster",
	}

	serverCmd.AddCommand(BalanceCommand(cfg))
	serverCmd.AddCommand(DeployCommand(cfg))
	serverCmd.AddCommand(SendMessageCommand(cfg))
	serverCmd.AddCommand(SendTokensCommand(cfg))
	serverCmd.AddCommand(TopUpCommand(cfg))
	serverCmd.AddCommand(NewCommand(cfg))

	return serverCmd
}
