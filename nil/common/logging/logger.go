package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

var (
	componentsFilter = make(map[string]bool)
	all              = true
	lock             = sync.Mutex{}
)

type ComponentFilterWriter struct {
	Writer io.Writer
	Name   string
}

func (w ComponentFilterWriter) Write(p []byte) (n int, err error) {
	var log map[string]any
	if err := json.Unmarshal(p, &log); err != nil {
		return 0, err
	}

	lock.Lock()
	defer lock.Unlock()

	enabled, found := componentsFilter[w.Name]
	if !found {
		enabled = all
	}
	if enabled {
		return w.Writer.Write(p)
	}

	return len(p), nil
}

func ApplyComponentsFilterEnv() {
	if logFilter := os.Getenv("NIL_LOG_FILTER"); logFilter != "" {
		ApplyComponentsFilter(logFilter)
	}
}

func ApplyComponentsFilter(filter string) {
	comps := strings.Split(filter, ":")

	lock.Lock()
	defer lock.Unlock()

	for _, comp := range comps {
		if comp == "" {
			continue
		}

		enabled := true
		if comp[0] == '-' {
			enabled = false
			comp = comp[1:]
		}

		if comp == "all" {
			all = enabled
			for k := range componentsFilter {
				componentsFilter[k] = enabled
			}
		} else {
			componentsFilter[comp] = enabled
		}
	}
}

func SetupGlobalLogger(level string) {
	check.PanicIfErr(TrySetupGlobalLevel(level))
	log.Logger = NewLogger("global", true).Logger()
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

//func NewLogger(component string) zerolog.Logger {
//	var logger zerolog.Logger
//	if isSystemd() {
//		logger = newJournalDLogger()
//	} else {
//		logger = newConsoleLogger(component)
//	}
//
//	return logger.With().
//		Bool("store_to_clickhouse", true).
//		Str(FieldComponent, component).
//		Caller().
//		Timestamp().
//		Logger()
//}

//func NewLoggerWithWriter(component string, writer io.Writer) zerolog.Logger {
//	logger := zerolog.New(ComponentFilterWriter{
//		Writer: writer,
//		Name:   component,
//	})
//	return logger.With().
//		Str(FieldComponent, component).
//		Caller().
//		Timestamp().
//		Logger()
//}

func newJournalDLogger() zerolog.Logger {
	return zerolog.New(journald.NewJournalDWriter())
}

func newConsoleLogger(component string) zerolog.Logger {
	noColor := os.Getenv("NO_COLOR") != "" || !term.IsTerminal(int(os.Stdout.Fd()))

	consoleWriter := zerolog.ConsoleWriter{
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
	}
	writer := ComponentFilterWriter{
		Writer: consoleWriter,
		Name:   component,
	}
	return zerolog.New(writer)
}
