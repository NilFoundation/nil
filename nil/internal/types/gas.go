package types

import (
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/NilFoundation/nil/nil/common/check"
)

const (
	OperationCosts       = 10_000_000 // 0.01 gwei transformed into wei
	ProofGenerationCosts = 10_000_000 // 0.01 gwei transformed into wei
	DefaultMaxGasInBlock = Gas(30_000_000)
)

var (
	DefaultGasPrice     = NewValueFromUint64(OperationCosts + ProofGenerationCosts)
	MaxFeePerGasDefault = DefaultGasPrice.Mul(DefaultGasPrice)
)

type Gas uint64

func (g Gas) Uint64() uint64 {
	return uint64(g)
}

func (g Gas) Add(other Gas) Gas {
	return Gas(g.Uint64() + other.Uint64())
}

func (g Gas) Sub(other Gas) Gas {
	return Gas(g.Uint64() - other.Uint64())
}

func (g Gas) Lt(other Gas) bool {
	return g.Uint64() < other.Uint64()
}

func (g Gas) ToValue(price Value) Value {
	res, overflow := g.ToValueOverflow(price)
	check.PanicIfNot(!overflow)
	return res
}

func (g Gas) ToValueOverflow(price Value) (Value, bool) {
	res, overflow := price.mulOverflow64(g.Uint64())
	return Value{res}, overflow
}

func (g Gas) MarshalText() ([]byte, error) {
	return []byte(g.String()), nil
}

func (g *Gas) UnmarshalText(input []byte) error {
	res, err := strconv.ParseUint(string(input), 10, 64)
	if err != nil {
		return err
	}
	*g = Gas(res)
	return nil
}

func (g Gas) MarshalJSON() ([]byte, error) {
	return []byte(`"` + g.String() + `"`), nil
}

func (g *Gas) UnmarshalJSON(input []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return &json.UnmarshalTypeError{Value: "non-string", Type: reflect.TypeOf(g)}
	}
	return g.UnmarshalText(input[1 : len(input)-1])
}

func (g Gas) String() string {
	return strconv.FormatUint(g.Uint64(), 10)
}

func (g *Gas) Set(value string) error {
	res, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	*g = Gas(res)
	return nil
}

func (Gas) Type() string {
	return "Gas"
}

func GasToValue(gas uint64) Value {
	return Gas(gas).ToValue(DefaultGasPrice)
}
