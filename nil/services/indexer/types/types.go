package types

import (
	"fmt"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type AddressAction struct {
	Hash      common.Hash         `json:"hash"`
	From      types.Address       `json:"from"`
	To        types.Address       `json:"to"`
	Amount    types.Value         `json:"amount"`
	Timestamp db.Timestamp        `json:"timestamp"`
	BlockId   types.BlockNumber   `json:"blockId"`
	Type      AddressActionKind   `json:"type"`
	Status    AddressActionStatus `json:"status"`
}

type AddressActionKind uint8

const (
	SendEth AddressActionKind = iota
	ReceiveEth
	SmartContractCall
)

func (k AddressActionKind) String() string {
	switch k {
	case SendEth:
		return "SendEth"
	case ReceiveEth:
		return "ReceiveEth"
	case SmartContractCall:
		return "SmartContractCall"
	}
	panic("unknown AddressActionKind")
}

func (k *AddressActionKind) Set(input string) error {
	switch input {
	case "SendEth":
		*k = SendEth
	case "ReceiveEth":
		*k = ReceiveEth
	case "SmartContractCall":
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

func (k AddressActionStatus) String() string {
	switch k {
	case Success:
		return "Success"
	case Failed:
		return "Failed"
	}
	panic("unknown AddressActionStatus")
}

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
