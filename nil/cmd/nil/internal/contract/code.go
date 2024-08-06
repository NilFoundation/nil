package contract

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

func GetCodeCommand(cfg *common.Config) *cobra.Command {
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

func runCode(_ *cobra.Command, args []string, cfg *common.Config) error {
	var address types.Address
	if err := address.Set(args[0]); err != nil {
		return err
	}

	service := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey)
	_, _ = service.GetCode(address)
	code, _ := service.GetCode(address)
	if !config.Quiet {
		fmt.Print("Contract code: ")
	}
	fmt.Println(code)
	return nil
}
