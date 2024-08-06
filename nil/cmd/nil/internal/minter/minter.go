package minter

import (
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "minter",
		Short: "Interact with minter on the cluster",
	}

	serverCmd.AddCommand(CreateCurrencyCommand(cfg))
	serverCmd.AddCommand(MintCurrencyCommand(cfg))
	serverCmd.AddCommand(WithdrawCurrencyCommand(cfg))

	return serverCmd
}
