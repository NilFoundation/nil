package logging

import (
	"fmt"
	"io"
	"reflect"

	"github.com/rs/zerolog"
)

var goToClickhouseTypes = map[string]struct {
	Abbreviation string
	Clickhouse   string
}{
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

func GetAbbreviationByInterface(v interface{}) string {
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

type CustomLogger struct {
	ctx zerolog.Context
}

func NewLogger(component string, storeToClick bool) CustomLogger {
	var logger zerolog.Logger
	if isSystemd() {
		logger = newJournalDLogger()
	} else {
		logger = newConsoleLogger(component)
	}

	customCtx := CustomLogger{ctx: logger.With()}

	return customCtx.
		Bool("store_to_clickhouse", storeToClick).
		Str(FieldComponent, component).
		Caller().
		Timestamp()
}

func NewLoggerWithWriter(component string, storeToClick bool, writer io.Writer) CustomLogger {
	logger := zerolog.New(ComponentFilterWriter{
		Writer: writer,
		Name:   component,
	})

	customCtx := CustomLogger{ctx: logger.With()}

	return customCtx.
		Bool("store_to_clickhouse", storeToClick).
		Str(FieldComponent, component).
		Caller().
		Timestamp()
}

func (c CustomLogger) Bool(key string, value bool) CustomLogger {
	return CustomLogger{ctx: c.ctx.Bool(key+GetAbbreviationByInterface(value), value)}
}

func (c CustomLogger) Str(key, value string) CustomLogger {
	return CustomLogger{ctx: c.ctx.Str(key+GetAbbreviationByInterface(value), value)}
}

// Bools adds the field key with val as a []bool to the logger CustomLogger.
func (c CustomLogger) Bools(key string, b []bool) CustomLogger {
	return CustomLogger{ctx: c.ctx.Bools(key+GetAbbreviationByInterface(b), b)}
}

// Int adds the field key with i as a int to the logger CustomLogger.
func (c CustomLogger) Int(key string, i int) CustomLogger {
	return CustomLogger{ctx: c.ctx.Int(key+GetAbbreviationByInterface(i), i)}
}

// Ints adds the field key with i as a []int to the logger CustomLogger.
func (c CustomLogger) Ints(key string, i []int) CustomLogger {
	return CustomLogger{ctx: c.ctx.Ints(key+GetAbbreviationByInterface(i), i)}
}

// Int8 adds the field key with i as a int8 to the logger CustomLogger.
func (c CustomLogger) Int8(key string, i int8) CustomLogger {
	return CustomLogger{ctx: c.ctx.Int8(key+GetAbbreviationByInterface(i), i)}
}

// Ints8 adds the field key with i as a []int8 to the logger CustomLogger.
func (c CustomLogger) Ints8(key string, i []int8) CustomLogger {
	return CustomLogger{ctx: c.ctx.Ints8(key+GetAbbreviationByInterface(i), i)}
}

// Int16 adds the field key with i as a int16 to the logger CustomLogger.
func (c CustomLogger) Int16(key string, i int16) CustomLogger {
	return CustomLogger{ctx: c.ctx.Int16(key+GetAbbreviationByInterface(i), i)}
}

// Ints16 adds the field key with i as a []int16 to the logger CustomLogger.
func (c CustomLogger) Ints16(key string, i []int16) CustomLogger {
	return CustomLogger{ctx: c.ctx.Ints16(key+GetAbbreviationByInterface(i), i)}
}

// Int32 adds the field key with i as a int32 to the logger CustomLogger.
func (c CustomLogger) Int32(key string, i int32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Int32(key+GetAbbreviationByInterface(i), i)}
}

// Ints32 adds the field key with i as a []int32 to the logger CustomLogger.
func (c CustomLogger) Ints32(key string, i []int32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Ints32(key+GetAbbreviationByInterface(i), i)}
}

// Int64 adds the field key with i as a int64 to the logger CustomLogger.
func (c CustomLogger) Int64(key string, i int64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Int64(key+GetAbbreviationByInterface(i), i)}
}

// Ints64 adds the field key with i as a []int64 to the logger CustomLogger.
func (c CustomLogger) Ints64(key string, i []int64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Ints64(key+GetAbbreviationByInterface(i), i)}
}

// Uint adds the field key with i as a uint to the logger CustomLogger.
func (c CustomLogger) Uint(key string, i uint) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uint(key+GetAbbreviationByInterface(i), i)}
}

// Uints adds the field key with i as a []uint to the logger CustomLogger.
func (c CustomLogger) Uints(key string, i []uint) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uints(key+GetAbbreviationByInterface(i), i)}
}

// Uint8 adds the field key with i as a uint8 to the logger CustomLogger.
func (c CustomLogger) Uint8(key string, i uint8) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uint8(key+GetAbbreviationByInterface(i), i)}
}

// Uints8 adds the field key with i as a []uint8 to the logger CustomLogger.
func (c CustomLogger) Uints8(key string, i []uint8) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uints8(key+GetAbbreviationByInterface(i), i)}
}

// Uint16 adds the field key with i as a uint16 to the logger CustomLogger.
func (c CustomLogger) Uint16(key string, i uint16) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uint16(key+GetAbbreviationByInterface(i), i)}
}

// Uints16 adds the field key with i as a []uint16 to the logger CustomLogger.
func (c CustomLogger) Uints16(key string, i []uint16) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uints16(key+GetAbbreviationByInterface(i), i)}
}

// Uint32 adds the field key with i as a uint32 to the logger CustomLogger.
func (c CustomLogger) Uint32(key string, i uint32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uint32(key+GetAbbreviationByInterface(key), i)}
}

// Uints32 adds the field key with i as a []uint32 to the logger CustomLogger.
func (c CustomLogger) Uints32(key string, i []uint32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uints32(key+GetAbbreviationByInterface(i), i)}
}

// Uint64 adds the field key with i as a uint64 to the logger CustomLogger.
func (c CustomLogger) Uint64(key string, i uint64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uint64(key+GetAbbreviationByInterface(i), i)}
}

// Uints64 adds the field key with i as a []uint64 to the logger CustomLogger.
func (c CustomLogger) Uints64(key string, i []uint64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Uints64(key+GetAbbreviationByInterface(i), i)}
}

// Float32 adds the field key with f as a float32 to the logger CustomLogger.
func (c CustomLogger) Float32(key string, f float32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Float32(key+GetAbbreviationByInterface(f), f)}
}

// Floats32 adds the field key with f as a []float32 to the logger CustomLogger.
func (c CustomLogger) Floats32(key string, f []float32) CustomLogger {
	return CustomLogger{ctx: c.ctx.Floats32(key+GetAbbreviationByInterface(f), f)}
}

// Float64 adds the field key with f as a float64 to the logger CustomLogger.
func (c CustomLogger) Float64(key string, f float64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Float64(key+GetAbbreviationByInterface(f), f)}
}

// Floats64 adds the field key with f as a []float64 to the logger CustomLogger.
func (c CustomLogger) Floats64(key string, f []float64) CustomLogger {
	return CustomLogger{ctx: c.ctx.Floats64(key+GetAbbreviationByInterface(f), f)}
}

func (c CustomLogger) Uint256(key, value string) CustomLogger {
	return CustomLogger{ctx: c.ctx.Str(key+GetAbbreviation("uint256"), value)}
}

func (c CustomLogger) Build() zerolog.Logger {
	return c.ctx.Logger()
}

// Caller adds caller information
func (c CustomLogger) Caller() CustomLogger {
	return CustomLogger{ctx: c.ctx.Caller()}
}

// Timestamp adds a timestamp
func (c CustomLogger) Timestamp() CustomLogger {
	return CustomLogger{ctx: c.ctx.Timestamp()}
}

// Build finalizes the logger
func (c CustomLogger) Logger() zerolog.Logger {
	return c.ctx.Logger()
}
