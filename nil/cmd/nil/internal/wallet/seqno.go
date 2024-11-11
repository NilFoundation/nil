package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func SeqnoCommand(cfg *common.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seqno",
		Short: "Get the seqno of the wallet whose address specified in config.address field",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeqno(cmd, args, cfg)
		},
		SilenceUsage: true,
	}

	return cmd
}

func runSeqno(_ *cobra.Command, _ []string, cfg *common.Config) error {
	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, nil)
	seqno, err := service.GetSeqno(cfg.Address)
	if err != nil {
		return err
	}
	if !config.Quiet {
		fmt.Print("Wallet seqno: ")
	}
	fmt.Println(seqno)
	return nil
}
