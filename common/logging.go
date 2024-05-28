package common

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

func SetupGlobalLogger() {
	log.Logger = *NewLogger("global")
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

func NewLogger(component string) *zerolog.Logger {
	const componentFieldName = "component"
	noColor := false

	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	} else if !term.IsTerminal(int(os.Stdout.Fd())) {
		noColor = true
	}

	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.DateTime,
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			componentFieldName,
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
		},
		FieldsExclude:    []string{componentFieldName},
		FormatFieldValue: makeComponentFormatter(noColor),
		NoColor:          noColor,
	}).
		Level(zerolog.TraceLevel).
		With().
		Str(componentFieldName, component).
		Caller().
		Timestamp().
		Logger()

	return &logger
}
