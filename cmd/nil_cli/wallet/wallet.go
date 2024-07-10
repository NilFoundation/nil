package wallet

import (
	"errors"
	"os"

	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/core/types"
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
				logger.Info().Msgf("No private key specified in config. Run `%s keygen` command to generate one.", os.Args[0])
				return errors.New("Private key is not specified in config")
			}
			if cfg.Address == types.EmptyAddress && cmd.Name() != "new" {
				logger.Info().Msgf("Valid wallet address is not specified in config. Run `%s config set address <address>` command to set.", os.Args[0])
				return errors.New("Valid wallet address is not specified in config")
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
