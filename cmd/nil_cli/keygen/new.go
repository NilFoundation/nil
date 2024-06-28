package keygen

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/spf13/cobra"
)

func NewCommand(keygen *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Generate a new key",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, keygen)
		},
		SilenceUsage: true,
	}
	return cmd
}

func runNew(_ *cobra.Command, _ []string, keygen *service.Service) error {
	if err := keygen.GenerateNewKey(); err != nil {
		return err
	}
	return nil
}
