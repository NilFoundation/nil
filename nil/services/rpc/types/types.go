package types

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var (
	ErrToAccNotFound  = errors.New("\"to\" account not found")
	ErrInvalidMessage = errors.New("invalid message")
)

// @component CallArgs callArgs string "The arguments for the message call."
// @componentprop Flags flags array true "The array of message flags."
// @componentprop From from string false "The address from which the message must be called."
// @componentprop FeeCredit feeCredit string true "The fee credit for the message."
// @componentprop Value value integer false "The message value."
// @componentprop Seqno seqno integer true "The sequence number of the message."
// @componentprop Data data string false "The encoded calldata."
// @componentprop Message message string false "The raw encoded input message."
// @component propr ChainId chainId integer "The chain id."
type CallArgs struct {
	Flags     types.MessageFlags `json:"flags,omitempty"`
	From      *types.Address     `json:"from,omitempty"`
	To        types.Address      `json:"to"`
	FeeCredit types.Value        `json:"feeCredit"`
	Value     types.Value        `json:"value,omitempty"`
	Seqno     types.Seqno        `json:"seqno"`
	Data      *hexutil.Bytes     `json:"data,omitempty"`
	Message   *hexutil.Bytes     `json:"input,omitempty"`
	ChainId   types.ChainId      `json:"chainId"`
}

func (args CallArgs) ToMessage() (*types.Message, error) {
	if args.Message != nil {
		// Try to decode default message
		msg := &types.Message{}
		if err := msg.UnmarshalSSZ(*args.Message); err == nil {
			return msg, nil
		}

		// Try to decode external message
		var extMsg types.ExternalMessage
		if err := extMsg.UnmarshalSSZ(*args.Message); err == nil {
			return extMsg.ToMessage(), nil
		}

		// Try to decode internal message payload
		var intMsg types.InternalMessagePayload
		if err := intMsg.UnmarshalSSZ(*args.Message); err == nil {
			var fromAddr types.Address
			if args.From != nil {
				fromAddr = *args.From
			}
			if intMsg.RefundTo.IsEmpty() {
				return nil, errors.New("refund address is empty")
			}
			return intMsg.ToMessage(fromAddr, args.Seqno), nil
		}
		return nil, ErrInvalidMessage
	}

	var data types.Code
	if args.Data != nil {
		data = types.Code(*args.Data)
	}
	msgFrom := args.To
	if args.From != nil {
		msgFrom = *args.From
	}
	return &types.Message{
		MessageDigest: types.MessageDigest{
			Flags:     args.Flags,
			ChainId:   types.DefaultChainId,
			Seqno:     args.Seqno,
			FeeCredit: args.FeeCredit,
			To:        args.To,
			Data:      data,
		},
		From:  msgFrom,
		Value: args.Value,
	}, nil
}

type OutMessage struct {
	MessageSSZ  []byte
	ForwardKind types.ForwardKind
	Data        []byte
	CoinsUsed   types.Value
	OutMessages []*OutMessage
	GasPrice    types.Value
	Error       string
	Logs        []*types.Log
	DebugLogs   []*types.DebugLog
}

type CallResWithGasPrice struct {
	Data           []byte
	CoinsUsed      types.Value
	OutMessages    []*OutMessage
	Error          string
	StateOverrides StateOverrides
	GasPrice       types.Value
	Logs           []*types.Log
	DebugLogs      []*types.DebugLog
}
