package types

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var (
	ErrBlockMismatch   = errors.New("block fetching mismatch")
	ErrBlockProcessing = errors.New("block processing error")
)

type BlockHashMismatchError struct {
	Expected      common.Hash
	Got           common.Hash
	LatestFetched types.BlockNumber
}

func (e *BlockHashMismatchError) Error() string {
	return fmt.Sprintf(
		"parent block hash mismatch: expected %s, got %s. Latest fetched block is %d", e.Expected, e.Got, e.LatestFetched,
	)
}

func (e *BlockHashMismatchError) Unwrap() error {
	return ErrBlockMismatch
}

var (
	ErrUnexpectedTaskType   = errors.New("unexpected task type")
	ErrBlockProofTaskFailed = errors.New("block proof task failed")
)

func UnexpectedTaskType(task *Task) error {
	return fmt.Errorf("%w: taskType=%d, taskId=%s", ErrUnexpectedTaskType, task.TaskType, task.Id)
}
