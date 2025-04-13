package logging

import (
	"fmt"
	"reflect"
	"time"

	"github.com/rs/zerolog"
)

const LogAbbreviationSize = 2

var goToClickhouseTypes = map[string]struct {
	Abbreviation string
	Clickhouse   string
}{
	"int":        {"II", "Int64"},
	"uint8":      {"U8", "UInt8"},
	"int8":       {"I8", "Int8"},
	"uint16":     {"U1", "UInt16"},
	"int16":      {"I1", "Int16"},
	"uint32":     {"U2", "UInt32"},
	"int32":      {"I2", "Int32"},
	"uint64":     {"U4", "UInt64"},
	"int64":      {"I4", "Int64"},
	"uint128":    {"UA", "UInt128"},
	"int128":     {"IA", "Int128"},
	"uint256":    {"UB", "UInt256"},
	"int256":     {"IB", "Int256"},
	"float32":    {"F3", "Float32"},
	"float64":    {"F6", "Float64"},
	"string":     {"S2", "String"},
	"bool":       {"B1", "Boolean"},
	"date":       {"D2", "Date"},
	"date32":     {"D3", "Date32"},
	"datetime":   {"DT", "DateTime"},
	"datetime64": {"D6", "DateTime64"},
	"enum8":      {"E8", "Enum8"},
	"enum16":     {"E1", "Enum16"},
	"array":      {"A1", "Array"},
	"tuple":      {"T1", "Tuple"},
	"map":        {"M1", "Map"},
	"nullable":   {"N1", "Nullable"},
	"json":       {"J1", "JSON"},
	"object":     {"O1", "Object"},
	"ipv4":       {"IP", "IPv4"},
	"ipv6":       {"I6", "IPv6"},
}

var abbreviationToClickhouse = make(map[string]struct {
	Gotype     string
	Clickhouse string
})

func init() {
	for k, v := range goToClickhouseTypes {
		if len(v.Abbreviation) != LogAbbreviationSize {
			panic(fmt.Sprintf("invalid abbreviation: %s, size must be %d", v.Abbreviation, LogAbbreviationSize))
		}
		abbreviationToClickhouse[v.Abbreviation] = struct {
			Gotype     string
			Clickhouse string
		}{
			Gotype:     k,
			Clickhouse: v.Clickhouse,
		}
	}
}

func GetClickhouseByAbbreviation(abbr string) (string, error) {
	chType, found := abbreviationToClickhouse[abbr]
	if !found {
		return "", fmt.Errorf("ClickHouse type not found for abbreviation %s", abbr)
	}
	return chType.Clickhouse, nil
}

func GetAbbreviationByInterface(v any) string {
	typeName := reflect.TypeOf(v).String()
	if val, found := goToClickhouseTypes[typeName]; found {
		return val.Abbreviation
	}
	return ""
}

func GetAbbreviation(abbr string) string {
	if val, found := goToClickhouseTypes[abbr]; found {
		return val.Abbreviation
	}
	return ""
}

func typedFields(fields any) any {
	switch fields := fields.(type) {
	case map[string]any:
		typed_fields := make(map[string]any)
		for key, value := range fields {
			typed_fields[key+GetAbbreviationByInterface(value)] = value
		}
		return typed_fields
	case []any:
		// zerolog ignores the last element if the number of elements is odd
		if n := len(fields); n&0x1 == 1 { // odd number
			fields = fields[:n-1]
		}
		typed_fields := make([]any, len(fields))
		copy(typed_fields, fields)
		for i, n := 0, len(fields); i < n; i += 2 {
			key, val := fields[i], fields[i+1]
			if key, ok := key.(string); ok {
				typed_fields[i] = key + GetAbbreviationByInterface(val)
			}
		}
		return typed_fields
	}
	return fields
}

type Context struct {
	ctx zerolog.Context
}

type Logger struct {
	logger zerolog.Logger
}

type Event struct {
	event *zerolog.Event
}

/////////////////// Context //////////////////////

func (c Context) Bool(key string, value bool) Context {
	return Context{ctx: c.ctx.Bool(key+GetAbbreviationByInterface(value), value)}
}

func (c Context) Str(key, value string) Context {
	return Context{ctx: c.ctx.Str(key+GetAbbreviationByInterface(value), value)}
}

// Bools adds the field key with val as a []bool to the logger CustomLogger.
func (c Context) Bools(key string, b []bool) Context {
	return Context{ctx: c.ctx.Bools(key+GetAbbreviationByInterface(b), b)}
}

// Int adds the field key with i as a int to the logger CustomLogger.
func (c Context) Int(key string, i int) Context {
	return Context{ctx: c.ctx.Int(key+GetAbbreviationByInterface(i), i)}
}

// Ints adds the field key with i as a []int to the logger CustomLogger.
func (c Context) Ints(key string, i []int) Context {
	return Context{ctx: c.ctx.Ints(key+GetAbbreviationByInterface(i), i)}
}

// Int8 adds the field key with i as a int8 to the logger CustomLogger.
func (c Context) Int8(key string, i int8) Context {
	return Context{ctx: c.ctx.Int8(key+GetAbbreviationByInterface(i), i)}
}

// Ints8 adds the field key with i as a []int8 to the logger CustomLogger.
func (c Context) Ints8(key string, i []int8) Context {
	return Context{ctx: c.ctx.Ints8(key+GetAbbreviationByInterface(i), i)}
}

// Int16 adds the field key with i as a int16 to the logger CustomLogger.
func (c Context) Int16(key string, i int16) Context {
	return Context{ctx: c.ctx.Int16(key+GetAbbreviationByInterface(i), i)}
}

// Ints16 adds the field key with i as a []int16 to the logger CustomLogger.
func (c Context) Ints16(key string, i []int16) Context {
	return Context{ctx: c.ctx.Ints16(key+GetAbbreviationByInterface(i), i)}
}

// Int32 adds the field key with i as a int32 to the logger CustomLogger.
func (c Context) Int32(key string, i int32) Context {
	return Context{ctx: c.ctx.Int32(key+GetAbbreviationByInterface(i), i)}
}

// Ints32 adds the field key with i as a []int32 to the logger CustomLogger.
func (c Context) Ints32(key string, i []int32) Context {
	return Context{ctx: c.ctx.Ints32(key+GetAbbreviationByInterface(i), i)}
}

// Int64 adds the field key with i as a int64 to the logger CustomLogger.
func (c Context) Int64(key string, i int64) Context {
	return Context{ctx: c.ctx.Int64(key+GetAbbreviationByInterface(i), i)}
}

// Ints64 adds the field key with i as a []int64 to the logger CustomLogger.
func (c Context) Ints64(key string, i []int64) Context {
	return Context{ctx: c.ctx.Ints64(key+GetAbbreviationByInterface(i), i)}
}

// Uint adds the field key with i as a uint to the logger CustomLogger.
func (c Context) Uint(key string, i uint) Context {
	return Context{ctx: c.ctx.Uint(key+GetAbbreviationByInterface(i), i)}
}

// Uints adds the field key with i as a []uint to the logger CustomLogger.
func (c Context) Uints(key string, i []uint) Context {
	return Context{ctx: c.ctx.Uints(key+GetAbbreviationByInterface(i), i)}
}

// Uint8 adds the field key with i as a uint8 to the logger CustomLogger.
func (c Context) Uint8(key string, i uint8) Context {
	return Context{ctx: c.ctx.Uint8(key+GetAbbreviationByInterface(i), i)}
}

// Uints8 adds the field key with i as a []uint8 to the logger CustomLogger.
func (c Context) Uints8(key string, i []uint8) Context {
	return Context{ctx: c.ctx.Uints8(key+GetAbbreviationByInterface(i), i)}
}

// Uint16 adds the field key with i as a uint16 to the logger CustomLogger.
func (c Context) Uint16(key string, i uint16) Context {
	return Context{ctx: c.ctx.Uint16(key+GetAbbreviationByInterface(i), i)}
}

// Uints16 adds the field key with i as a []uint16 to the logger CustomLogger.
func (c Context) Uints16(key string, i []uint16) Context {
	return Context{ctx: c.ctx.Uints16(key+GetAbbreviationByInterface(i), i)}
}

// Uint32 adds the field key with i as a uint32 to the logger CustomLogger.
func (c Context) Uint32(key string, i uint32) Context {
	return Context{ctx: c.ctx.Uint32(key+GetAbbreviationByInterface(key), i)}
}

// Uints32 adds the field key with i as a []uint32 to the logger CustomLogger.
func (c Context) Uints32(key string, i []uint32) Context {
	return Context{ctx: c.ctx.Uints32(key+GetAbbreviationByInterface(i), i)}
}

// Uint64 adds the field key with i as a uint64 to the logger CustomLogger.
func (c Context) Uint64(key string, i uint64) Context {
	return Context{ctx: c.ctx.Uint64(key+GetAbbreviationByInterface(i), i)}
}

// Uints64 adds the field key with i as a []uint64 to the logger CustomLogger.
func (c Context) Uints64(key string, i []uint64) Context {
	return Context{ctx: c.ctx.Uints64(key+GetAbbreviationByInterface(i), i)}
}

// Float32 adds the field key with f as a float32 to the logger CustomLogger.
func (c Context) Float32(key string, f float32) Context {
	return Context{ctx: c.ctx.Float32(key+GetAbbreviationByInterface(f), f)}
}

// Floats32 adds the field key with f as a []float32 to the logger CustomLogger.
func (c Context) Floats32(key string, f []float32) Context {
	return Context{ctx: c.ctx.Floats32(key+GetAbbreviationByInterface(f), f)}
}

// Float64 adds the field key with f as a float64 to the logger CustomLogger.
func (c Context) Float64(key string, f float64) Context {
	return Context{ctx: c.ctx.Float64(key+GetAbbreviationByInterface(f), f)}
}

// Floats64 adds the field key with f as a []float64 to the logger CustomLogger.
func (c Context) Floats64(key string, f []float64) Context {
	return Context{ctx: c.ctx.Floats64(key+GetAbbreviationByInterface(f), f)}
}

func (c Context) Hex(key string, val []byte) Context {
	return Context{ctx: c.ctx.Hex(key+GetAbbreviationByInterface(val), val)}
}

func (c Context) Dur(key string, d time.Duration) Context {
	return Context{ctx: c.ctx.Dur(key+GetAbbreviation("float64"), d)}
}

func (c Context) Any(key string, val any) Context {
	return Context{ctx: c.ctx.Any(key+GetAbbreviation("json"), val)}
}

func (c Context) Interface(key string, val any) Context {
	return c.Any(key, val)
}

func (c Context) Fields(fields any) Context {
	return Context{ctx: c.ctx.Fields(typedFields(fields))}
}

func (c Context) Uint256(key, value string) Context {
	return Context{ctx: c.ctx.Str(key+GetAbbreviation("uint256"), value)}
}

// Caller adds caller information
func (c Context) Caller() Context {
	return Context{ctx: c.ctx.Caller()}
}

func (c Context) CallerWithSkipFrameCount(skipFrameCount int) Context {
	return Context{ctx: c.ctx.CallerWithSkipFrameCount(skipFrameCount)}
}

// Timestamp adds a timestamp
func (c Context) Timestamp() Context {
	return Context{ctx: c.ctx.Timestamp()}
}

func (c Context) Stringer(key string, val fmt.Stringer) Context {
	return Context{ctx: c.ctx.Stringer(key+GetAbbreviation("string"), val)}
}

func (c Context) Logger() Logger {
	return Logger{logger: c.ctx.Logger()}
}

/////////////////// Event //////////////////////

func (e *Event) Bool(key string, value bool) *Event {
	e.event.Bool(key+GetAbbreviationByInterface(value), value)
	return e
}

func (e *Event) Str(key, value string) *Event {
	e.event.Str(key+GetAbbreviationByInterface(value), value)
	return e
}

func (e *Event) Bools(key string, b []bool) *Event {
	e.event.Bools(key+GetAbbreviationByInterface(b), b)
	return e
}

func (e *Event) Int(key string, i int) *Event {
	e.event.Int(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Ints(key string, i []int) *Event {
	e.event.Ints(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Int8(key string, i int8) *Event {
	e.event.Int8(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Ints8(key string, i []int8) *Event {
	e.event.Ints8(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Int16(key string, i int16) *Event {
	e.event.Int16(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Ints16(key string, i []int16) *Event {
	e.event.Ints16(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Int32(key string, i int32) *Event {
	e.event.Int32(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Ints32(key string, i []int32) *Event {
	e.event.Ints32(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Int64(key string, i int64) *Event {
	e.event.Int64(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Ints64(key string, i []int64) *Event {
	e.event.Ints64(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uint(key string, i uint) *Event {
	e.event.Uint(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uints(key string, i []uint) *Event {
	e.event.Uints(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uint8(key string, i uint8) *Event {
	e.event.Uint8(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uints8(key string, i []uint8) *Event {
	e.event.Uints8(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uint16(key string, i uint16) *Event {
	e.event.Uint16(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uints16(key string, i []uint16) *Event {
	e.event.Uints16(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uint32(key string, i uint32) *Event {
	e.event.Uint32(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uints32(key string, i []uint32) *Event {
	e.event.Uints32(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uint64(key string, i uint64) *Event {
	e.event.Uint64(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Uints64(key string, i []uint64) *Event {
	e.event.Uints64(key+GetAbbreviationByInterface(i), i)
	return e
}

func (e *Event) Float32(key string, f float32) *Event {
	e.event.Float32(key+GetAbbreviationByInterface(f), f)
	return e
}

func (e *Event) Floats32(key string, f []float32) *Event {
	e.event.Floats32(key+GetAbbreviationByInterface(f), f)
	return e
}

func (e *Event) Float64(key string, f float64) *Event {
	e.event.Float64(key+GetAbbreviationByInterface(f), f)
	return e
}

func (e *Event) Floats64(key string, f []float64) *Event {
	e.event.Floats64(key+GetAbbreviationByInterface(f), f)
	return e
}

func (e *Event) Uint256(key, value string) *Event {
	e.event.Str(key+GetAbbreviation("uint256"), value)
	return e
}

func (e *Event) Stringer(key string, val fmt.Stringer) *Event {
	e.event.Stringer(key+GetAbbreviation("string"), val)
	return e
}

func (e *Event) Err(err error) *Event {
	e.event.Err(err)
	return e
}

func (e *Event) RawJSON(key string, value []byte) *Event {
	e.event.RawJSON(key+GetAbbreviation("json"), value)
	return e
}

func (e *Event) Dur(key string, d time.Duration) *Event {
	e.event.Dur(key+GetAbbreviation("float64"), d)
	return e
}

func (e *Event) Hex(key string, val []byte) *Event {
	e.event.Hex(key+GetAbbreviation("string"), val)
	return e
}

func (e *Event) Fields(fields any) *Event {
	e.event.Fields(typedFields(fields))
	return e
}

func (e *Event) Any(key string, val any) *Event {
	e.event.Any(key+GetAbbreviation("json"), val)
	return e
}

func (e *Event) Interface(key string, val any) *Event {
	e.Any(key, val)
	return e
}

func (e *Event) Msg(msg string) {
	e.event.CallerSkipFrame(1).Msg(msg)
}

func (e *Event) Msgf(format string, v ...any) {
	e.event.CallerSkipFrame(1).Msgf(format, v...)
}

func (e *Event) Send() {
	e.event.CallerSkipFrame(1).Send()
}

/////////////////// Logger //////////////////////

func (l Logger) With() Context {
	return Context{ctx: l.logger.With()}
}

func (l *Logger) GetLevel() zerolog.Level {
	return l.logger.GetLevel()
}

// Trace
func (l *Logger) Trace() *Event {
	return &Event{event: l.logger.Trace()} //nolint:zerologlint
}

// Debug
func (l *Logger) Debug() *Event {
	return &Event{event: l.logger.Debug()} //nolint:zerologlint
}

// Info
func (l *Logger) Info() *Event {
	return &Event{event: l.logger.Info()} //nolint:zerologlint
}

// Warn
func (l *Logger) Warn() *Event {
	return &Event{event: l.logger.Warn()} //nolint:zerologlint
}

// Error
func (l *Logger) Error() *Event {
	return &Event{event: l.logger.Error()} //nolint:zerologlint
}

// Fatal
func (l *Logger) Fatal() *Event {
	return &Event{event: l.logger.Fatal()} //nolint:zerologlint
}

// Panic
func (l *Logger) Panic() *Event {
	return &Event{event: l.logger.Panic()} //nolint:zerologlint
}

// Log
func (l *Logger) Log() *Event {
	return &Event{event: l.logger.Log()} //nolint:zerologlint
}

// Err
func (l *Logger) Err(err error) *Event {
	return &Event{event: l.logger.Err(err)} //nolint:zerologlint
}

// WithLevel
func (l *Logger) WithLevel(level zerolog.Level) *Event {
	return &Event{event: l.logger.WithLevel(level)} //nolint:zerologlint
}

// Level
func (l Logger) Level(lvl zerolog.Level) Logger {
	return Logger{logger: l.logger.Level(lvl)}
}
