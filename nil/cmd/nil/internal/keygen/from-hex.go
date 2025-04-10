package keygen

import (
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func FromHexCommand(keygen *cliservice.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-hex",
		Short: "Generate a key from a provided hex private key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFromHex(cmd, args, keygen)
		},
		SilenceUsage: true,
	}
	return cmd
}

func runFromHex(_ *cobra.Command, args []string, keygen *cliservice.Service) error {
	return keygen.GenerateKeyFromHex(args[0])
}
