package types

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var (
	ErrToAccNotFound      = errors.New("\"to\" account not found")
	ErrInvalidTransaction = errors.New("invalid transaction")
)

// @component CallArgs callArgs string "The arguments for the transaction call."
// @componentprop Flags flags array true "The array of transaction flags."
// @componentprop From from string false "The address from which the transaction must be called."
// @componentprop FeeCredit feeCredit string true "The fee credit for the transaction."
// @componentprop Value value integer false "The transaction value."
// @componentprop Seqno seqno integer true "The sequence number of the transaction."
// @componentprop Data data string false "The encoded calldata."
// @componentprop Transaction transaction string false "The raw encoded input transaction."
// @component propr ChainId chainId integer "The chain id."
type CallArgs struct {
	Flags       types.TransactionFlags `json:"flags,omitempty"`
	From        *types.Address         `json:"from,omitempty"`
	To          types.Address          `json:"to"`
	Fee         types.FeePack          `json:"fee,omitempty"`
	Value       types.Value            `json:"value,omitempty"`
	Seqno       types.Seqno            `json:"seqno"`
	Data        *hexutil.Bytes         `json:"data,omitempty"`
	Transaction *hexutil.Bytes         `json:"input,omitempty"`
	ChainId     types.ChainId          `json:"chainId"`
}

func (args CallArgs) ToTransaction() (*types.Transaction, error) {
	if args.Transaction != nil {
		// Try to decode default transaction
		txn := &types.Transaction{}
		if err := txn.UnmarshalNil(*args.Transaction); err == nil {
			return txn, nil
		}

		// Try to decode external transaction
		var extTxn types.ExternalTransaction
		if err := extTxn.UnmarshalNil(*args.Transaction); err == nil {
			return extTxn.ToTransaction(), nil
		}

		// Try to decode internal transaction payload
		var intTxn types.InternalTransactionPayload
		if err := intTxn.UnmarshalNil(*args.Transaction); err == nil {
			var fromAddr types.Address
			if args.From != nil {
				fromAddr = *args.From
			}
			if intTxn.RefundTo.IsEmpty() {
				return nil, errors.New("refund address is empty")
			}
			tx := intTxn.ToTransaction(fromAddr, args.Seqno)

			// For internal messages, we need to set MaxFeePerGas from the input args.
			tx.MaxFeePerGas = args.Fee.MaxFeePerGas

			return tx, nil
		}
		return nil, ErrInvalidTransaction
	}

	var data types.Code
	if args.Data != nil {
		data = types.Code(*args.Data)
	}
	txnFrom := args.To
	if args.From != nil {
		txnFrom = *args.From
	}
	return &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   args.Flags,
			ChainId: types.DefaultChainId,
			Seqno:   args.Seqno,
			FeePack: args.Fee,
			To:      args.To,
			Data:    data,
		},
		From:  txnFrom,
		Value: args.Value,
	}, nil
}

type OutTransaction struct {
	TransactionSSZ  []byte
	ForwardKind     types.ForwardKind
	Data            []byte
	CoinsUsed       types.Value
	OutTransactions []*OutTransaction
	BaseFee         types.Value
	Error           string
	Logs            []*types.Log
	DebugLogs       []*types.DebugLog
}

type CallResWithGasPrice struct {
	Data            []byte
	CoinsUsed       types.Value
	OutTransactions []*OutTransaction
	Error           string
	StateOverrides  StateOverrides
	BaseFee         types.Value
	Logs            []*types.Log
	DebugLogs       []*types.DebugLog
}
