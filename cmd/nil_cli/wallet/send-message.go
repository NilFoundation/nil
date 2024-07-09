package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func SendMessageCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-message [address] [bytecode or method] [args...]",
		Short: "Send a message to the smart contract via the wallet",
		Long:  "Send a message to the smart contract with specified bytecode or command via the wallet",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSend(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)

	params.amount = types.Value{}
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"Amount of tokens to send",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Wait for receipt",
	)

	params.gasLimit = 100_000
	cmd.Flags().Var(
		&params.gasLimit,
		gasLimitFlag,
		"Gas limit",
	)

	cmd.Flags().StringArrayVar(&params.currencies,
		tokenFlag,
		nil,
		"Token to transfer in format '<currencyId>=<amount>', can be used multiple times",
	)

	return cmd
}

func runSend(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := service.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1:])
	if err != nil {
		return err
	}

	currencies, err := common.ParseCurrencies(params.currencies)
	if err != nil {
		return err
	}

	msgHash, err := service.RunContract(cfg.Address, calldata, params.gasLimit, params.amount, currencies, address)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(cfg.Address.ShardId(), msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}

	fmt.Println(msgHash)
	return nil
}
