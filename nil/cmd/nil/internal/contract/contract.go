package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetCommand(cfg *common.Config) *cobra.Command {
	params := &contractParams{
		Params: &common.Params{},
	}
	serverCmd := &cobra.Command{
		Use:   "contract [address]",
		Short: "Interact with a contract on the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContract(cmd, args, cfg, params)
		},
	}
	setContractFlags(serverCmd, params)

	serverCmd.AddCommand(
		GetAddressCommand(cfg),
		GetBalanceCommand(cfg),
		GetTokensCommand(cfg),
		GetCodeCommand(cfg),
		GetCallReadonlyCommand(cfg),
		GetDeployCommand(cfg),
		GetSendExternalTransactionCommand(cfg),
		GetEstimateFeeCommand(cfg),
		GetTopUpCommand(cfg),
		GetSeqnoCommand(),
	)

	return serverCmd
}

func setContractFlags(cmd *cobra.Command, params *contractParams) {
	cmd.Flags().StringVar(&params.blockId, "block", "latest", "Block number, hash or tag")
}

func runContract(cmd *cobra.Command, args []string, cfg *common.Config, params *contractParams) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, nil)

	debugRPCContract, _ := service.GetDebugContract(address, params.blockId)

	if debugRPCContract != nil {
		contract := new(types.SmartContract)
		if err := contract.UnmarshalSSZ(debugRPCContract.Contract); err != nil {
			return err
		}

		fmt.Println("Hash:", contract.Hash())
		fmt.Println("Address:", contract.Address.String())
		fmt.Println("Balance:", contract.Balance.Uint256.String())
		fmt.Println("Seqno:", contract.Seqno.String())
		fmt.Println("ExtSeqno:", contract.ExtSeqno.String())
		fmt.Println("StorageRoot:", contract.StorageRoot.String())
	}

	return nil
}
