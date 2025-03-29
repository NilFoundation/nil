package testaide

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// Now represents a fixed point in time in the UTC timezone used as a fake `time.Now()` for tests.
var Now = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

// NewTestClock creates a new instance of test clock initialized with a fixed time defined in Now.
func NewTestClock() *clockwork.FakeClock {
	return clockwork.NewFakeClockAt(Now)
}

// ResetTestClock resets the test clock to the time defined in Now.
func ResetTestClock(clock *clockwork.FakeClock) {
	now := clock.Now()
	diff := Now.Sub(now)
	clock.Advance(diff)
}
