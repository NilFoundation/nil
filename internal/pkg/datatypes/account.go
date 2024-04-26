package datatypes

import (
	mtree "github.com/NilFoundation/nil/internal/pkg/merkle_tree"
	mpt "github.com/keybase/go-merkle-tree"
)

type SmartContract struct {
	Addr        []byte   `hashable:"" storable:""`
	StorageRoot mpt.Hash `hashable:"" storable:""`
	Storage     *mtree.TreeWrapper
}
