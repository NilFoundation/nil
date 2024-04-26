package datatypes

import (
	mtree "github.com/NilFoundation/nil/internal/pkg/merkle_tree"
	mpt "github.com/keybase/go-merkle-tree"
)

type Block struct {
	Hash               mpt.Hash `storable:""`
	Id                 uint64   `hashable:"" storable:""`
	PrevBlock          mpt.Hash `hashable:"" storable:""`
	SmartContractsRoot mpt.Hash `hashable:"" storable:""`

	SmartContracts *mtree.TreeWrapper
}
