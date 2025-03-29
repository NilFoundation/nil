package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"golang.org/x/term"
)

var GlobalLogger Logger

var (
	componentsFilter = make(map[string]bool)
	all              = true
	lock             = sync.RWMutex{}
)

type ComponentFilterWriter struct {
	Writer          io.Writer
	Name            string
	IsConsoleWriter bool
}

func (w ComponentFilterWriter) Write(p []byte) (n int, err error) {
	var log map[string]any
	if err := json.Unmarshal(p, &log); err != nil {
		return 0, err
	}

	lock.RLock()
	enabled, found := componentsFilter[w.Name]
	lock.RUnlock()

	if !found {
		enabled = all
	}
	if !enabled {
		return len(p), nil
	}
	if w.IsConsoleWriter {
		// Remove the type suffix from the log fields in case of console logger.
		logWithoutType := make(map[string]any)
		for k, v := range log {
			switch k {
			case zerolog.CallerFieldName,
				zerolog.TimestampFieldName,
				zerolog.MessageFieldName,
				zerolog.LevelFieldName,
				FieldComponent:
				logWithoutType[k] = v
				continue
			}
			logWithoutType[k[:len(k)-LogAbbreviationSize]] = v
		}
		res, err := json.Marshal(logWithoutType)
		if err != nil {
			return 0, err
		}
		return w.Writer.Write(res)
	}
	return w.Writer.Write(p)
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
	if err := TrySetupGlobalLevel(level); err != nil {
		panic(err)
	}
	GlobalLogger = NewLogger("global")
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

func NewLoggerWithStore(component string, storeToClick bool) Logger {
	var logger zerolog.Logger
	if isSystemd() {
		logger = newJournalDLogger()
	} else {
		logger = newConsoleLogger(component)
	}

	customCtx := Context{ctx: logger.With()}
	if isSystemd() {
		customCtx = customCtx.Bool(FieldStoreToClickhouse, storeToClick)
	}

	return customCtx.
		Str(FieldComponent, component).
		Caller().
		Timestamp().
		Logger()
}

func NewLogger(component string) Logger {
	return NewLoggerWithStore(component, true)
}

func NewLoggerWithWriterStore(component string, storeToClick bool, writer io.Writer) Logger {
	logger := zerolog.New(ComponentFilterWriter{
		Writer: writer,
		Name:   component,
	})

	ctx := Context{ctx: logger.With()}

	return ctx.
		Bool(FieldStoreToClickhouse, storeToClick).
		Str(FieldComponent, component).
		Caller().
		Timestamp().
		Logger()
}

func NewLoggerWithWriter(component string, writer io.Writer) Logger {
	return NewLoggerWithWriterStore(component, true, writer)
}

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
		Writer:          consoleWriter,
		Name:            component,
		IsConsoleWriter: true,
	}
	return zerolog.New(writer)
}

func Nop() Logger {
	return Logger{logger: zerolog.Nop()}
}

func NewFromZerolog(logger zerolog.Logger) Logger {
	return Logger{logger: logger}
}
