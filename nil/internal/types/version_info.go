package types

import (
	"fmt"
	"reflect"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/serialization"
)

type VersionInfo struct {
	Version common.Hash `json:"version,omitempty"`
}

// interfaces
var (
	_ serialization.NilMarshaler   = new(VersionInfo)
	_ serialization.NilUnmarshaler = new(VersionInfo)
)

var SchemesInsideDb = []any{
	SmartContract{},
	Block{},
	Transaction{},
	ExternalTransaction{},
	InternalTransactionPayload{},
	AsyncContext{},
	CollatorState{},
}

const SchemeVersionInfoKey = "SchemeVersionInfo"

func NewVersionInfo() *VersionInfo {
	var res []byte
	for _, curStruct := range SchemesInsideDb {
		v := reflect.ValueOf(curStruct)
		for i := range v.NumField() {
			t := v.Type()
			res = append(res, common.KeccakHash([]byte(t.String())).Bytes()...)

			name := t.Field(i).Name
			res = append(res, common.KeccakHash([]byte(name)).Bytes()...)

			value := fmt.Sprintf("%v", v.Field(i).Interface())
			res = append(res, common.KeccakHash([]byte(value)).Bytes()...)
		}
	}
	return &VersionInfo{Version: common.KeccakHash(res)}
}

func (v *VersionInfo) UnmarshalNil(buf []byte) error {
	return v.UnmarshalSSZ(buf)
}

func (v VersionInfo) MarshalNil() ([]byte, error) {
	return v.MarshalSSZ()
}
