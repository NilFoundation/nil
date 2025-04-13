package common

import (
	"bytes"
	"encoding"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// PValueSlice is an auxiliary type for typed parsing of command line parameters with a list of values.
// Implements the pflag.Value interface.
type PValueSlice[T interface {
	~*S
	pflag.Value
	encoding.TextMarshaler
	encoding.TextUnmarshaler
}, S any] []S

// Checking compliance with the pflag.Value interface.
var (
	_ pflag.Value = (*Hash)(nil)
	_ pflag.Value = (*PValueSlice[*Hash, Hash])(nil)
)

func (s *PValueSlice[T, S]) Set(value string) error {
	parts, err := readAsCSV(value)
	if err != nil {
		return err
	}

	*s = nil
	for _, part := range parts {
		val := new(S)
		if err := T(val).Set(part); err != nil {
			return err
		}
		*s = append(*s, *val)
	}
	return nil
}

func (s *PValueSlice[T, S]) String() string {
	values := make([]string, len(*s))
	for j, val := range *s {
		values[j] = T(&val).String()
	}
	str, err := WriteAsCSV(values)
	if err != nil {
		return err.Error()
	}
	return str
}

func (s *PValueSlice[T, S]) Type() string {
	return fmt.Sprintf("PValueSlice[%s]", T(new(S)).Type())
}

func (s PValueSlice[T, S]) MarshalYAML() (any, error) {
	values := make([]string, len(s))
	for j, val := range s {
		values[j] = T(&val).String()
	}
	return values, nil
}

func (s *PValueSlice[T, S]) UnmarshalYAML(value *yaml.Node) error {
	var values []string
	if err := value.Decode(&values); err != nil {
		return err
	}

	*s = nil
	for _, val := range values {
		item := new(S)
		if err := T(item).Set(val); err != nil {
			return err
		}
		*s = append(*s, *item)
	}
	return nil
}

// Copy of private functions from pflag used in stringSliceValue implementation.
func readAsCSV(val string) ([]string, error) {
	if val == "" {
		return []string{}, nil
	}
	stringReader := strings.NewReader(val)
	csvReader := csv.NewReader(stringReader)
	return csvReader.Read()
}

func WriteAsCSV(vals []string) (string, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	if err := w.Write(vals); err != nil {
		return "", err
	}
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n"), nil
}
