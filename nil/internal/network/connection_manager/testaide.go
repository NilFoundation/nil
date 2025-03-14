//go:build test

package connection_manager

import (
	"math"

	"github.com/jonboulle/clockwork"
)

// CalculateDecayPercent (t, p) calculates the percentage of the decrease
// that needs to be used on each tick, so that for ticks the reputation
// falls approximately to the share of p (p from 0 to 1).
// The whole number of percent is returned (rounding up).
func CalculateDecayPercent(t int, p float64) uint {
	// fraction = 1 - p^(1/T), This is the part of the reputation (in shares),
	// which needs to be "cut" in one tick.
	fraction := 1.0 - math.Pow(p, 1.0/float64(t))

	// We convert a share of interest and round up
	percent := uint(math.Ceil(fraction * 100.0))

	if percent < 1 {
		percent = 1
	}
	return percent
}

func SetClock(config *Config, clock clockwork.Clock) {
	config.clock = clock
}
