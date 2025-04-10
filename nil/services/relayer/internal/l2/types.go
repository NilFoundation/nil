package l2

import (
	"math/big"

	"github.com/NilFoundation/nil/nil/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

type Event struct {
	BlockNumber    uint64            `json:"blockNumber"`
	Hash           ethcommon.Hash    `json:"eventHash"`
	SequenceNumber uint64            `json:"sequenceNumber"`
	FeePack        types.FeePack     `json:"fee"`
	L2Limit        types.Value       `json:"l2Limit"`
	Sender         ethcommon.Address `json:"sender"`
	Target         ethcommon.Address `json:"target"`
	Message        []byte            `json:"message"`
	Nonce          *big.Int          `json:"nonce"`
	Type           uint8             `json:"messageType"`
	ExpiryTime     *big.Int          `json:"expiryTime"`
}
