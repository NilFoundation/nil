package contract

import (
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/common"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "contract",
		Short: "Interact with contract on the cluster",
	}

	serverCmd.AddCommand(GetAddressCommand(cfg))
	serverCmd.AddCommand(GetBalanceCommand(cfg))
	serverCmd.AddCommand(GetCurrenciesCommand(cfg))
	serverCmd.AddCommand(GetCodeCommand(cfg))
	serverCmd.AddCommand(GetCallReadonlyCommand(cfg))
	serverCmd.AddCommand(GetDeployCommand(cfg))
	serverCmd.AddCommand(GetSendExternalMessageCommand(cfg))

	return serverCmd
}
