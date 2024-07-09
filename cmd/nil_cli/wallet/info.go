package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/spf13/cobra"
)

func InfoCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Returns wallet's address and public key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return infoBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func infoBalance(_ *cobra.Command, _ []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)
	addr, pub, err := service.GetInfo(cfg.Address)
	if err != nil {
		return err
	}

	if !config.Quiet {
		fmt.Print("Wallet address: ")
	}
	fmt.Println(addr)

	if !config.Quiet {
		fmt.Print("Public key: ")
	}
	fmt.Println(pub)

	return nil
}
