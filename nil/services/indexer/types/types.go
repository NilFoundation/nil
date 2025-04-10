package types

import (
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type AddressAction struct {
	Hash    common.Hash         `json:"hash"`
	From    types.Address       `json:"from"`
	To      types.Address       `json:"to"`
	Amount  types.Value         `json:"amount"`
	BlockId types.BlockNumber   `json:"blockId"`
	Type    AddressActionKind   `json:"type"`
	Status  AddressActionStatus `json:"status"`
}

type AddressActionKind uint8

const (
	SendEth AddressActionKind = iota
	ReceiveEth
	SmartContractCall
)

func (k *AddressActionKind) Set(input string) error {
	switch strings.ToLower(input) {
	case "sendeth":
		*k = SendEth
	case "receiveeth":
		*k = ReceiveEth
	case "smartcontractcall":
		*k = SmartContractCall
	default:
		return fmt.Errorf("unknown AddressActionKind: %s", input)
	}
	return nil
}

type AddressActionStatus uint8

const (
	Success AddressActionStatus = iota
	Failed
)

func (k *AddressActionStatus) Set(input string) error {
	switch input {
	case "Success":
		*k = Success
	case "Failed":
		*k = Failed
	default:
		return fmt.Errorf("unknown AddressActionStatus: %s", input)
	}
	return nil
}
