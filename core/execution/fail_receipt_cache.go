package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

type ReceiptWithError struct {
	Receipt *types.Receipt
	Error   error
}

// todo: this is a temporary solution, we shouldn't store errors for unpaid failures
var FailureReceiptCache, _ = lru.New[common.Hash, ReceiptWithError](1024)

func AddFailureReceipt(hash common.Hash, to types.Address, err error) {
	FailureReceiptCache.Add(hash, ReceiptWithError{
		Receipt: &types.Receipt{
			Success:         false,
			MsgHash:         hash,
			ContractAddress: to,
		},
		Error: err,
	})

	sharedLogger.Debug().
		Err(err).
		Stringer(logging.FieldMessageHash, hash).
		Stringer(logging.FieldMessageTo, to).
		Msg("Cached non-authorized fail receipt.")
}
