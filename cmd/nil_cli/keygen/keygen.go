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
	serverCmd := &cobra.Command{
		Use:          "keygen",
		Short:        "Generate a new key or generate a key from a provided hex private key",
		PreRunE:      runPreRun,
		Run:          runCommand,
		SilenceUsage: true,
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&params.newKey,
		newFlag,
		false,
		"Generate a new key",
	)
	cmd.Flags().StringVar(
		&params.hexPrivateKey,
		fromHexFlag,
		"",
		"Generate a key from a provided hex private key",
	)
}

func runCommand(cmd *cobra.Command, _ []string) {
	keygen := service.NewService(&rpc.Client{}, nil)

	var err error
	if params.newKey {
		err = keygen.GenerateNewKey()
	} else {
		err = keygen.GenerateKeyFromHex(params.hexPrivateKey)
	}

	if err != nil {
		logger.Fatal().Msg(err.Error())
	}

	privateKey := keygen.GetPrivateKey()
	logger.Info().Msgf("Pivate key: %v", privateKey)

	if err := common.PatchConfig(map[string]interface{}{
		common.PrivateKeyField: privateKey,
	}, false); err != nil {
		logger.Error().Err(err).Msg("failed to update private key in config file")
	}
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	return params.initRawParams()
}
