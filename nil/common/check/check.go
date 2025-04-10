package check

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
)

// These functions are meant to simplify panicking in the code
// Always consider returning errors instead of panicking!
//
// Generally, you need the simpler versions: PanicIfNot and PanicIfErr.
// If you use the f-versions (PanicIfNotf and LogAndPanicIfErrf),
// the message should be informative and should have runtime-defined arguments.
// Panic dumps a stack trace, so messages without specific data do not add anything.
//
// As a rule of thumb, if you wish to use the function with a custom message,
// consider returning a wrapped error instead.

// PanicIfNot panics on false (use as simple assert).
func PanicIfNot(flag bool) {
	if !flag {
		panic("requirement not met")
	}
}

// PanicIff panics on true with the given message.
func PanicIff(flag bool, format string, args ...any) {
	PanicIfNotf(!flag, format, args...)
}

// PanicIfNotf panics on false with the given message.
func PanicIfNotf(flag bool, format string, args ...any) {
	if !flag {
		panic(fmt.Sprintf(format, args...))
	}
}

// PanicIfErr calls panic(err) if err is not nil.
func PanicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

// PanicIfNotCancelledErr panics if the provided error is non-nil and not a context.Canceled error.
func PanicIfNotCancelledErr(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}

	panic(err)
}

// LogAndPanicIfErrf logs the error with the provided logger and message and panics if err is not nil.
func LogAndPanicIfErrf(err error, logger logging.Logger, format string, args ...any) {
	if err != nil {
		l := logger.With().CallerWithSkipFrameCount(3).Logger()
		l.Error().Err(err).Msgf(format, args...)
		panic(err)
	}
}
