package keygen

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/spf13/cobra"
)

func FromHexCommand(keygen *service.Service) *cobra.Command {
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

func runFromHex(_ *cobra.Command, args []string, keygen *service.Service) error {
	if err := keygen.GenerateKeyFromHex(args[0]); err != nil {
		return err
	}
	return nil
}
