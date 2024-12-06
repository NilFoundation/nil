package message

import (
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/spf13/cobra"
)

func GetInternalMessageCommand() *cobra.Command {
	var (
		kind                   types.MessageKind = types.ExecutionMessageKind
		bounce                 bool
		feeCredit              types.Value       = types.NewValueFromUint64(100_000)
		forwardKind            types.ForwardKind = types.ForwardKindNone
		to, refundTo, bounceTo types.Address
		value                  types.Value
		data                   hexutil.Bytes
	)

	encodeCmd := &cobra.Command{
		Use:   "encode-internal",
		Short: "Encode an internal message",
		Args:  cobra.ExactArgs(0),
		RunE: func(_ *cobra.Command, args []string) error {
			message := &types.InternalMessagePayload{
				Kind:        kind,
				Bounce:      bounce,
				FeeCredit:   feeCredit,
				ForwardKind: forwardKind,
				To:          to,
				RefundTo:    refundTo,
				BounceTo:    bounceTo,
				Currency:    nil,
				Value:       value,
				Data:        types.Code(data),
			}

			messageStr, err := json.MarshalIndent(message, "", " ")
			if err != nil {
				return err
			}

			messageSsz, err := message.MarshalSSZ()
			if err != nil {
				return err
			}

			messageSszHex := hexutil.Encode(messageSsz)

			if !common.Quiet {
				fmt.Println("Message:")
				fmt.Println(string(messageStr))
				fmt.Print("Result: ")
			}
			fmt.Println(messageSszHex)

			if !common.Quiet {
				fmt.Printf("Hash: %x\n", message.ToMessage(types.EmptyAddress, types.Seqno(0)).Hash())
			}
			return nil
		},
		SilenceUsage: true,
	}

	encodeCmd.Flags().Var(
		&kind,
		kindFlag,
		"The message type [execution|deploy|refund]",
	)

	encodeCmd.Flags().BoolVarP(
		&bounce,
		bounceFlag, bounceFlagShort,
		false,
		"Define whether the \"bounce\" flag is set",
	)

	encodeCmd.Flags().Var(
		&feeCredit,
		feeCreditFlag,
		"The fee credit",
	)

	encodeCmd.Flags().Var(
		&forwardKind,
		forwardKindFlag,
		"The gas forward kind [remaining|percentage|value|none]",
	)

	encodeCmd.Flags().Var(
		&to,
		toFlag,
		"The destination address for the message",
	)

	encodeCmd.Flags().Var(
		&refundTo,
		refundToFlag,
		"The redund address",
	)

	encodeCmd.Flags().Var(
		&bounceTo,
		bounceToFlag,
		"The bounce address",
	)

	encodeCmd.Flags().Var(
		&value,
		valueFlag,
		"The message value",
	)

	encodeCmd.Flags().Var(
		&data,
		dataFlag,
		"The message data",
	)
	check.PanicIfErr(encodeCmd.MarkFlagRequired(dataFlag))

	return encodeCmd
}
