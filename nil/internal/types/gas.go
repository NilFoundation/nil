package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
	return json.Marshal(g.Hex())
}

func (g *Gas) UnmarshalJSON(input []byte) error {
	// trim quotes
	var str string
	if err := json.Unmarshal(input, &str); err != nil {
		return fmt.Errorf("gas unmarshal failed for value: %s (%w)", input, err)
	}
	gas, err := GasFromHex(str)
	if err != nil {
		return err
	}
	*g = gas
	return nil
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

func (g Gas) Hex() string {
	return fmt.Sprintf("0x%x", uint64(g))
}

func GasFromHex(s string) (Gas, error) {
	if !strings.HasPrefix(s, "0x") {
		return 0, fmt.Errorf("invalid hex format: %s", s)
	}
	gas, err := strconv.ParseUint(s[2:], 16, 64)
	if err != nil {
		return 0, err
	}
	return Gas(gas), nil
}
