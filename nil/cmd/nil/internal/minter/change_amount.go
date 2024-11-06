package minter

import (
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func ChangeCurrencyAmountCommand(cfg *common.Config, mint bool) *cobra.Command {
	method := "Burn"
	if mint {
		method = "Mint"
	}

	cmd := &cobra.Command{
		Use:   strings.ToLower(method) + "-currency [address] [amount]",
		Short: method + " a custom currency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChangeCurrencyAmount(cmd, args, cfg, mint)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runChangeCurrencyAmount(_ *cobra.Command, args []string, cfg *common.Config, mint bool) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	var amount types.Value
	if err := amount.Set(args[1]); err != nil {
		return err
	}

	txHash, err := service.ChangeCurrencyAmount(address, amount, mint)
	if err != nil {
		return err
	}
	if !config.Quiet {
		if mint {
			fmt.Printf("Minted %v amount of currency to %v, TX Hash: ", amount, address)
		} else {
			fmt.Printf("Burned %v amount of currency from %v, TX Hash: ", amount, address)
		}
	}
	fmt.Println(txHash)
	return nil
}
