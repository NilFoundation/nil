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

	serverCmd.AddCommand(TopUpCommand(cfg))

	return serverCmd
}
