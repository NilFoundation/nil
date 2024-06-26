package contract

import (
	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetCodeCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code [address]",
		Short: "Returns a smart contract code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCode(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runCode(_ *cobra.Command, args []string, cfg *config.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	_, _ = service.GetCode(address)
	return nil
}
