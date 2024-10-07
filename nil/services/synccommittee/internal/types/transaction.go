package types

import (
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type PrunedTransaction struct {
	Flags types.MessageFlags
	Seqno hexutil.Uint64
	From  types.Address
	To    types.Address
	Value types.Value
	Data  hexutil.Bytes
}
