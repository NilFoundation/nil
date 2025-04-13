package encode

import (
	"io"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type BatchEncoder interface {
	Encode(in *types.PrunedBatch, out io.Writer) error
}
