package transaction

import (
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/spf13/cobra"
)

type bytes struct {
	hexutil.Bytes
}

func (b *bytes) Set(s string) error {
	var err error
	b.Bytes, err = hexutil.Decode(s)
	return err
}

func (b bytes) Type() string {
	return "bytes"
}

func GetInternalTransactionCommand() *cobra.Command {
	var (
		kind                   = types.ExecutionTransactionKind
		feeCredit              = types.NewValueFromUint64(100_000)
		bounce                 bool
		forwardKind            types.ForwardKind = types.ForwardKindNone
		to, refundTo, bounceTo types.Address
		value                  types.Value
		data                   bytes
	)

	encodeCmd := &cobra.Command{
		Use:   "encode-internal",
		Short: "Encode an internal transaction",
		Args:  cobra.ExactArgs(0),
		RunE: func(_ *cobra.Command, args []string) error {
			transaction := &types.InternalTransactionPayload{
				Kind:        kind,
				Bounce:      bounce,
				FeeCredit:   feeCredit,
				ForwardKind: forwardKind,
				To:          to,
				RefundTo:    refundTo,
				BounceTo:    bounceTo,
				Token:       nil,
				Value:       value,
				Data:        types.Code(data.Bytes),
			}

			transactionStr, err := json.MarshalIndent(transaction, "", " ")
			if err != nil {
				return err
			}

			transactionBin, err := transaction.MarshalNil()
			if err != nil {
				return err
			}

			transactionHex := hexutil.Encode(transactionBin)

			if !common.Quiet {
				fmt.Println("Transaction:")
				fmt.Println(string(transactionStr))
				fmt.Print("Result: ")
			}
			fmt.Println(transactionHex)

			if !common.Quiet {
				fmt.Printf("Hash: %x\n", transaction.ToTransaction(types.EmptyAddress, types.Seqno(0)).Hash())
			}
			return nil
		},
		SilenceUsage: true,
	}

	encodeCmd.Flags().Var(
		&kind,
		kindFlag,
		"The transaction type [execution|deploy|refund]",
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
		"The destination address for the transaction",
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
		"The transaction value",
	)

	encodeCmd.Flags().Var(
		&data,
		dataFlag,
		"The transaction data",
	)
	check.PanicIfErr(encodeCmd.MarkFlagRequired(dataFlag))

	return encodeCmd
}
