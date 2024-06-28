package keygen

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("keygenCommand")

func GetCommand() *cobra.Command {
	keygen := service.NewService(&rpc.Client{}, nil)

	keygenCmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate a new key or generate a key from a provided hex private key",
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			privateKey := keygen.GetPrivateKey()
			logger.Info().Msgf("Pivate key: %v", privateKey)

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
