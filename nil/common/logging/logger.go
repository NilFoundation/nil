package logging

import (
	"fmt"
	"os"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

func SetupGlobalLogger(level string) {
	check.PanicIfErr(TrySetupGlobalLevel(level))
	log.Logger = NewLogger("global")
}

func TrySetupGlobalLevel(level string) error {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(l)
	return nil
}

// defaults to INFO
func SetLogSeverityFromEnv() {
	if lvl, err := zerolog.ParseLevel(os.Getenv("LOG_LEVEL")); err != nil {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(lvl)
	}
}

func makeBold(str any, disabled bool) string {
	const colorBold = 1

	if disabled {
		return fmt.Sprintf("%s", str)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", colorBold, str)
}

func makeComponentFormatter(noColor bool) zerolog.Formatter {
	return func(c any) string {
		return makeBold(fmt.Sprintf("[%s]\t", c), noColor)
	}
}

// isSystemd returns true if the process is running under systemd
func isSystemd() bool {
	return os.Getenv("INVOCATION_ID") != ""
}

func NewLogger(component string) zerolog.Logger {
	var logger zerolog.Logger
	if isSystemd() {
		logger = newJournalDLogger()
	} else {
		logger = newConsoleLogger()
	}

	return logger.With().
		Str(FieldComponent, component).
		Caller().
		Timestamp().
		Logger()
}

func newJournalDLogger() zerolog.Logger {
	return zerolog.New(journald.NewJournalDWriter())
}

func newConsoleLogger() zerolog.Logger {
	noColor := os.Getenv("NO_COLOR") != "" || !term.IsTerminal(int(os.Stdout.Fd()))
	return zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.DateTime,
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			FieldComponent,
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
		},
		FieldsExclude:    []string{FieldComponent},
		FormatFieldValue: makeComponentFormatter(noColor),
		NoColor:          noColor,
	})
}
