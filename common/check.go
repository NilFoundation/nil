package common

import (
	"github.com/rs/zerolog"
)

// Require panics on false.
// Can be used in the code in the places where you want to ensure some result without much error handling.
func Require(flag bool) {
	if !flag {
		// Maybe collect stack trace and optionally add a message from the caller.
		panic("requirement not met")
	}
}

// FatalIf logs the error with the provided logger and message and panics.
// It is no-op if the error is nil.
// It uses the default logger if logger is nil.
func FatalIf(err error, logger zerolog.Logger, format string, args ...interface{}) {
	if err == nil {
		return
	}

	l := logger.With().CallerWithSkipFrameCount(3).Logger()
	l.Err(err).Msgf(format, args...)
	panic(err)
}
