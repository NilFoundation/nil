package execution

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

type ReceiptWithError struct {
	Receipt *types.Receipt
	Error   error
}

// todo: this is a temporary solution, we shouldn't store errors for unpaid failures
var FailureReceiptCache, _ = lru.New[common.Hash, ReceiptWithError](1024)

func AddFailureReceipt(hash common.Hash, to types.Address, execResult *ExecutionResult) {
	FailureReceiptCache.Add(hash, ReceiptWithError{
		Receipt: &types.Receipt{
			Status:          execResult.Error.Status,
			Success:         false,
			MsgHash:         hash,
			ContractAddress: to,
		},
		Error: execResult.Error,
	})

	sharedLogger.Debug().
		Err(execResult.Error).
		Stringer(logging.FieldMessageHash, hash).
		Stringer(logging.FieldMessageTo, to).
		Msg("Cached non-authorized fail receipt.")
}
