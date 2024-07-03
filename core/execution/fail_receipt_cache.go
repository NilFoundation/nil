package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

type ReceiptWithError struct {
	Receipt *types.Receipt
	Error   error
}

var FailureReceiptCache, _ = lru.New[common.Hash, ReceiptWithError](1024)
