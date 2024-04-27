package types

import (
	common "github.com/NilFoundation/nil/common"
	mpt "github.com/keybase/go-merkle-tree"
)

type SmartContract struct {
	Addr        []byte   `hashable:"" storable:""`
	StorageRoot mpt.Hash `hashable:"" storable:""`
	Storage     *common.TreeWrapper
}
