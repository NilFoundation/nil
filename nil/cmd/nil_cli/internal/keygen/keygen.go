package keygen

import (
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/cmd/nil_cli/internal/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("keygenCommand")

func GetCommand() *cobra.Command {
	keygen := cliservice.NewService(&rpc.Client{}, nil)

	keygenCmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate a new key or generate a key from a provided hex private key",
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			privateKey := keygen.GetPrivateKey()
			logger.Info().Msgf("Private key: %v", privateKey)

			if err := common.PatchConfig(map[string]interface{}{
				common.PrivateKeyField: privateKey,
			}, false); err != nil {
				logger.Error().Err(err).Msg("failed to update private key in config file")
			}
			return nil
		},
		SilenceUsage: true,
	}

	keygenCmd.AddCommand(NewCommand(keygen))
	keygenCmd.AddCommand(FromHexCommand(keygen))
	return keygenCmd
}
