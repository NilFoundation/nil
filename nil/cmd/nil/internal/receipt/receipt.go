package receipt

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	libcommon "github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("receiptCommand")

func GetCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:          "receipt [hash]",
		Short:        "Retrieve a receipt from the cluster",
		Args:         cobra.ExactArgs(1),
		RunE:         runCommand,
		SilenceUsage: true,
	}

	return serverCmd
}

func runCommand(cmd *cobra.Command, args []string) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), nil, nil)

	var hash libcommon.Hash
	if err := hash.Set(args[0]); err != nil {
		return err
	}

	if hash != libcommon.EmptyHash {
		receipt, err := service.FetchReceiptByHashJson(hash)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch the receipt")
			return err
		}
		if !common.Quiet {
			fmt.Print("Receipt data: ")
		}
		fmt.Println(string(receipt))
		return nil
	}
	return errors.New("empty hash")
}
