package types

import (
	common "github.com/NilFoundation/nil/common"
	mpt "github.com/keybase/go-merkle-tree"
)

type Block struct {
	Hash               mpt.Hash `storable:""`
	Id                 uint64   `hashable:"" storable:""`
	PrevBlock          mpt.Hash `hashable:"" storable:""`
	SmartContractsRoot mpt.Hash `hashable:"" storable:""`

	SmartContracts *common.TreeWrapper
}
