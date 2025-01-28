package types

import (
	"fmt"
	"reflect"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
)

type VersionInfo struct {
	Version common.Hash `json:"version,omitempty"`
}

// interfaces
var (
	_ fastssz.Marshaler   = new(VersionInfo)
	_ fastssz.Unmarshaler = new(VersionInfo)
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
			res = append(res, common.PoseidonHash([]byte(t.String())).Bytes()...)

			name := t.Field(i).Name
			res = append(res, common.PoseidonHash([]byte(name)).Bytes()...)

			value := fmt.Sprintf("%v", v.Field(i).Interface())
			res = append(res, common.PoseidonHash([]byte(value)).Bytes()...)
		}
	}
	return &VersionInfo{Version: common.PoseidonHash(res)}
}

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path version_info.go -include ../../common/hash.go,../../common/length.go --objs VersionInfo
