package contract

import (
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "contract",
		Short: "Interact with contract on the cluster",
	}

	serverCmd.AddCommand(GetBalanceCommand(cfg))
	serverCmd.AddCommand(GetCodeCommand(cfg))
	serverCmd.AddCommand(GetCallReadonlyCommand(cfg))
	serverCmd.AddCommand(GetDeployCommand(cfg))
	serverCmd.AddCommand(GetSendExternalMessageCommand(cfg))

	return serverCmd
}
