package check

import (
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
func PanicIff(flag bool, format string, args ...interface{}) {
	PanicIfNotf(!flag, format, args...)
}

// PanicIfNotf panics on false with the given message.
func PanicIfNotf(flag bool, format string, args ...interface{}) {
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

// LogAndPanicIfErrf logs the error with the provided logger and message and panics if err is not nil.
func LogAndPanicIfErrf(err error, logger logging.Logger, format string, args ...interface{}) {
	if err != nil {
		l := logger.With().CallerWithSkipFrameCount(3).Logger()
		l.Error().Err(err).Msgf(format, args...)
		panic(err)
	}
}
