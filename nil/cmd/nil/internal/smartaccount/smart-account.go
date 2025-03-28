package smartaccount

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("smart-account")

func GetCommand(cfg *common.Config) *cobra.Command {
	var serverCmd *cobra.Command

	serverCmd = &cobra.Command{
		Use:   "smart-account",
		Short: "Interact with the smart account set in the config file",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if parent := serverCmd.Parent(); parent != nil {
				if parent.PersistentPreRunE != nil {
					if err := parent.PersistentPreRunE(parent, args); err != nil {
						return err
					}
				}
			}
			if cfg.PrivateKey == nil {
				return config.MissingKeyError(config.PrivateKeyField, logger)
			}
			if cfg.Address == types.EmptyAddress && cmd.Name() != "new" {
				return config.MissingKeyError(config.AddressField, logger)
			}
			return nil
		},
	}

	serverCmd.AddCommand(
		BalanceCommand(cfg),
		DeployCommand(cfg),
		InfoCommand(cfg),
		SendTransactionCommand(cfg),
		SendTokensCommand(cfg),
		SeqnoCommand(cfg),
		TopUpCommand(cfg),
		NewCommand(cfg),
		CallReadonlyCommand(cfg),
		GetEstimateFeeCommand(cfg),
	)

	return serverCmd
}
