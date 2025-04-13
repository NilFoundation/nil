package l1

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type depositType = uint8

const (
	depositTypeERC20 depositType = 0
	depositTypeETH   depositType = 1
)

type Event struct {
	// ID
	Hash common.Hash `json:"eventHash"` // from MessageHash field

	// Block related info
	BlockNumber uint64      `json:"blkNum"`
	BlockHash   common.Hash `json:"blkHash"`

	// Used for proper ordering events while sending to L2
	// Assigned locally (and sequentially for each fetched from the L1 event)
	// Does not guarantee order for events collected by different relayer instances
	SequenceNumber uint64 `json:"sequenceNumber"`

	// Payload
	Sender             common.Address `json:"sender"`
	Target             common.Address `json:"target"`
	Nonce              *big.Int       `json:"nonce"`
	Message            []byte         `json:"message"`
	Type               uint8          `json:"messageType"`
	CreatedAt          *big.Int       `json:"createdAt"`
	ExpiryTime         *big.Int       `json:"expiryTime"`
	L2FeeRefundAddress common.Address `json:"l2FeeRefundAddress"`
	FeeCreditData      FeeCreditData  `json:"feeCreditData"`
}

func (ev *Event) validate() error {
	if ev.Type != depositTypeERC20 &&
		ev.Type != depositTypeETH {
		return fmt.Errorf("%w: unexpected deposit type: %d", ErrInvalidEvent, ev.Type)
	}
	if ev.ExpiryTime == nil || ev.Nonce == nil {
		return fmt.Errorf("%w: expiry time (%v) and nonce (%v) fields cannot be empty",
			ErrInvalidEvent, ev.ExpiryTime, ev.Nonce)
	}
	return nil
}

type FeeCreditData struct {
	NilGasLimit          *big.Int `json:"nilGasLimit"`
	MaxFeePerGas         *big.Int `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *big.Int `json:"maxPriorityFeePerGas"`
	FeeCredit            *big.Int `json:"feeCredit"`
}

type ProcessedBlock struct {
	BlockHash   common.Hash `json:"blkHash"`
	BlockNumber uint64      `json:"blkNum"`
	// TODO add all needed fields needed for last processed block info storage
}
