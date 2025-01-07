package testaide

import (
	"time"

	"github.com/NilFoundation/nil/nil/common"
)

// Now represents a fixed point in time in the UTC timezone used as a fake `time.Now()` for tests.
var Now = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

// NewTestTimer creates a new instance of test timer initialized with a fixed time defined in Now.
func NewTestTimer() *common.TestTimerImpl {
	return common.NewTestTimerFromTime(Now)
}
