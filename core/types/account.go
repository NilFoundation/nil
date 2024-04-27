package types

import (
	common "github.com/NilFoundation/nil/common"
)

type SmartContract struct {
	Addr        []byte      `hashable:"" storable:""`
	StorageRoot common.Hash `hashable:"" storable:""`
}
