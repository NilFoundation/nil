package db

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
	mpt "github.com/keybase/go-merkle-tree"
)

type valueFactory struct{}

func (valueFactory) Construct() interface{} {
	return struct {
		Value string
	}{}
}

type poseidonHasher struct{}

func (poseidonHasher) Hash(data []byte) mpt.Hash {
	return poseidon.Sum(data)
}

func UpdateTree(tree *mpt.Tree, key, value string) error {
	kv := mpt.KeyValuePair{Key: poseidon.Sum([]byte(key)), Value: []byte(value)}
	return tree.Upsert(context.TODO(), kv, nil)
}

type MerkleTree struct {
	tree *mpt.Tree
	eng  mpt.StorageEngine
}

func (tree *MerkleTree) Root() (common.Hash, error) {
	hash, err := tree.eng.LookupRoot(context.TODO())
	if err != nil {
		return common.Hash{}, err
	}

	return common.Hash(hash[:]), nil
}

func (tree *MerkleTree) Find(key common.Hash) (interface{}, error) {
	val, _, err := tree.tree.Find(context.TODO(), mpt.Hash(key[:]))

	return val, err
}

func (tree *MerkleTree) Upsert(key common.Hash, value interface{}) error {
	return tree.tree.Upsert(context.TODO(), mpt.KeyValuePair{Key: mpt.Hash(key[:]), Value: value}, nil)
}

func GetMerkleTree(tx Tx, root common.Hash) *MerkleTree {
	cfg := mpt.NewConfig(poseidonHasher{}, mpt.ChildIndex(4), mpt.ChildIndex(1), valueFactory{})
	eng := GetEngine(tx, root)
	return &MerkleTree{tree: mpt.NewTree(eng, cfg), eng: eng}
}
