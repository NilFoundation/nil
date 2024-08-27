package wallet

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("wallet")

func GetCommand(cfg *common.Config) *cobra.Command {
	var serverCmd *cobra.Command

	serverCmd = &cobra.Command{
		Use:   "wallet",
		Short: "Interact with wallet on the cluster",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if parent := serverCmd.Parent(); parent != nil {
				if parent.PersistentPreRunE != nil {
					if err := parent.PersistentPreRunE(parent, args); err != nil {
						return err
					}
				}
			}
			if cfg.PrivateKey == nil {
				return common.MissingKeyError(common.PrivateKeyField, logger)
			}
			if cfg.Address == types.EmptyAddress && cmd.Name() != "new" {
				return common.MissingKeyError(common.AddressField, logger)
			}
			return nil
		},
	}

	serverCmd.AddCommand(BalanceCommand(cfg))
	serverCmd.AddCommand(DeployCommand(cfg))
	serverCmd.AddCommand(InfoCommand(cfg))
	serverCmd.AddCommand(SendMessageCommand(cfg))
	serverCmd.AddCommand(SendTokensCommand(cfg))
	serverCmd.AddCommand(TopUpCommand(cfg))
	serverCmd.AddCommand(NewCommand(cfg))
	serverCmd.AddCommand(CallReadonlyCommand(cfg))

	return serverCmd
}
