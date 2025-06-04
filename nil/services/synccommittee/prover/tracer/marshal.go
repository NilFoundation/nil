package tracer

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type MarshalMode int

const (
	MarshalModeBinary MarshalMode = 1 << iota
	MarshalModeJSON
)

func (m MarshalMode) Add(other MarshalMode) MarshalMode {
	return m | other
}

func (m MarshalMode) Has(v MarshalMode) bool {
	return (m & v) > 0
}

func (m MarshalMode) getMarshallers() map[string]marshaler {
	ret := make(map[string]marshaler)
	m.iterateEnabled(func(mode MarshalMode) {
		fn, ok := marshalModeToMarshaller[mode]
		check.PanicIfNotf(ok, "no marshaler defined for mode %X", mode)
		ret[mode.String()] = fn
	})
	return ret
}

func (m MarshalMode) String() string {
	var ret []string
	m.iterateEnabled(func(mode MarshalMode) {
		str, ok := marshalModeToStrMapping[mode]
		if ok {
			ret = append(ret, str)
		}
	})
	return strings.Join(ret, ",")
}

func (m MarshalMode) iterateEnabled(fn func(MarshalMode)) {
	for i := 1; i <= int(m); i <<= 1 {
		mode := MarshalMode(i)
		if m.Has(mode) {
			fn(mode)
		}
	}
}

func MarshalModeFromString(s string) (MarshalMode, error) {
	var ret MarshalMode
	for _, submode := range strings.Split(s, ",") {
		m, ok := marshalModeFromStrMapping[strings.ToLower(submode)]
		if !ok {
			return 0, errors.New("no marshaller defined for mode " + submode)
		}
		ret = ret.Add(m)
	}
	return ret, nil
}

type (
	marshaler   func(proto.Message) ([]byte, error)
	unmarshaler func([]byte, proto.Message) error
)

var (
	marshalModeToStrMapping = map[MarshalMode]string{
		MarshalModeBinary: "bin",
		MarshalModeJSON:   "json",
	}

	marshalModeFromStrMapping = common.ReverseMap(marshalModeToStrMapping)

	marshalModeToMarshaller = map[MarshalMode]marshaler{
		MarshalModeBinary: proto.Marshal,
		MarshalModeJSON: func(txn proto.Message) ([]byte, error) {
			opts := protojson.MarshalOptions{
				Multiline: true,
			}
			return opts.Marshal(txn)
		},
	}

	marshalModeToUnmarshaller = map[MarshalMode]unmarshaler{
		MarshalModeBinary: proto.Unmarshal,
		MarshalModeJSON:   protojson.Unmarshal,
	}
)

func marshalToFile[Msg proto.Message](txn Msg, marshalFunc marshaler, filename string) error {
	data, err := marshalFunc(txn)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o600)
}

func unmarshalFromFile[Msg proto.Message](filename string, unmarshalFunc unmarshaler, out Msg) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return unmarshalFunc(data, out)
}

func marshalJSONToFile[T any](value T, filename string) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o600)
}
