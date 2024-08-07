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

var SchemesInsideDb = []interface{}{SmartContract{}, Block{}, Message{}, ExternalMessage{}, InternalMessagePayload{}}

const SchemeVersionInfoKey = "SchemeVersionInfo"

func NewVersionInfo() *VersionInfo {
	var res []byte
	for _, curStruct := range SchemesInsideDb {
		v := reflect.ValueOf(curStruct)
		for i := range v.NumField() {
			name := v.Type().Field(i).Name
			res = append(res, common.PoseidonHash([]byte(name)).Bytes()...)

			value := v.Field(i).Interface()
			valueStr := fmt.Sprintf("%v", value)
			res = append(res, common.PoseidonHash([]byte(valueStr)).Bytes()...)
		}
	}
	return &VersionInfo{Version: common.PoseidonHash(res)}
}
