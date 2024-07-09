package wallet

import (
	"errors"

	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/spf13/cobra"
)

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
				return errors.New("No private key specified in config. Run `nil_cli keygen` command to generate one.")
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

	return serverCmd
}
