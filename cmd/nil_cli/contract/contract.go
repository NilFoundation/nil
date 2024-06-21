package contract

import (
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "contract",
		Short: "Interact with contract on the cluster",
	}

	serverCmd.AddCommand(GetBalanceCommand(cfg))
	serverCmd.AddCommand(GetCodeCommand(cfg))
	serverCmd.AddCommand(GetCallCommand(cfg))
	serverCmd.AddCommand(GetDeployCommand(cfg))
	serverCmd.AddCommand(GetSendCommand(cfg))
	serverCmd.AddCommand(GetTransferCommand(cfg))

	return serverCmd
}
