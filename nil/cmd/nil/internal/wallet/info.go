package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func InfoCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get the address and the public key of the wallet set in the config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return infoBalance(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func infoBalance(_ *cobra.Command, _ []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)
	addr, pub, err := service.GetInfo(cfg.Address)
	if err != nil {
		return err
	}

	if !common.Quiet {
		fmt.Print("Wallet address: ")
	}
	fmt.Println(addr)

	if !common.Quiet {
		fmt.Print("Public key: ")
	}
	fmt.Println(pub)

	return nil
}
