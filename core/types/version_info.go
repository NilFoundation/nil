package types

import (
	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
)

type VersionInfo struct {
	Version common.Hash `json:"version,omitempty"`
}

// interfaces
var (
	_ fastssz.Marshaler   = new(VersionInfo)
	_ fastssz.Unmarshaler = new(VersionInfo)
)

var SchemesInsideDb = []common.Hash{new(SmartContract).Hash(), new(Block).Hash(), new(Message).Hash()}

const SchemeVersionInfoKey = "SchemeVersionInfo"

func NewVersionInfo() *VersionInfo {
	var res []byte
	for _, hash := range SchemesInsideDb {
		res = append(res, hash.Bytes()...)
	}
	return &VersionInfo{Version: common.PoseidonHash(res)}
}
