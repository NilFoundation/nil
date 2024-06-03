package keygen

import (
	"fmt"

	keyManagerService "github.com/NilFoundation/nil/cli/services/keygen"
	"github.com/NilFoundation/nil/common"
	"github.com/spf13/cobra"
)

var logger = common.NewLogger("keygenCommand")

func GetCommand() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "keygen",
		Short:   "Generate a new key or generate a key from a provided hex private key",
		PreRunE: runPreRun,
		Run:     runCommand,
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
	keygen := keyManagerService.NewService()

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
	logger.Info().Msg(fmt.Sprintf("Pivate key: %v", privateKey))
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	return params.initRawParams()
}
