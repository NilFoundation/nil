package core

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
)

var ErrBlockHashMismatch = errors.New("prev block hash mismatch")

type BlockHashMismatchError struct {
	Expected common.Hash
	Got      common.Hash
}

func (e *BlockHashMismatchError) Error() string {
	return fmt.Sprintf("Prev block hash mismatch: expected %s, got %s", e.Expected, e.Got)
}

func (e *BlockHashMismatchError) Unwrap() error {
	return ErrBlockHashMismatch
}
