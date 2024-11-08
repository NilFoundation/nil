package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func SendMessageCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-message [address] [bytecode or method] [args...]",
		Short: "Send a message to a smart contract via the wallet",
		Long:  "Send a message to the smart contract with the specified bytecode or command via the wallet",
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
		"The path to the ABI file",
	)

	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"The amount of default tokens to send",
	)

	cmd.Flags().BoolVar(
		&params.noWait,
		noWaitFlag,
		false,
		"Define whether the command should wait for the receipt",
	)

	cmd.Flags().Var(
		&params.feeCredit,
		feeCreditFlag,
		"The fee credit for message processing",
	)

	cmd.Flags().StringArrayVar(&params.currencies,
		tokenFlag,
		nil,
		"The custom currencies to transfer in as a map 'currencyId=amount', can be set multiple times",
	)

	return cmd
}

func runSend(_ *cobra.Command, args []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)

	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	calldata, err := common.PrepareArgs(params.abiPath, args[1], args[2:])
	if err != nil {
		return err
	}

	currencies, err := common.ParseCurrencies(params.currencies)
	if err != nil {
		return err
	}

	msgHash, err := service.RunContract(cfg.Address, calldata, params.feeCredit, params.amount, currencies, address)
	if err != nil {
		return err
	}

	if !params.noWait {
		if _, err := service.WaitForReceipt(msgHash); err != nil {
			return err
		}
	}

	if !config.Quiet {
		fmt.Print("Message hash: ")
	}

	fmt.Println(msgHash)
	return nil
}
