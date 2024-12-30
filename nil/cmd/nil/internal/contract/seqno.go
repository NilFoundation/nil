package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetSeqnoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "seqno [address]",
		Short:        "Get the seqno of a smart contract",
		Args:         cobra.ExactArgs(1),
		RunE:         runSeqno,
		SilenceUsage: true,
	}

	return cmd
}

func runSeqno(cmd *cobra.Command, args []string) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), nil, nil)
	seqno, err := service.GetSeqno(address)
	if err != nil {
		return err
	}
	if !common.Quiet {
		fmt.Print("Contract seqno: ")
	}
	fmt.Println(seqno)
	return nil
}
