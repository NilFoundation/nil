package db

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/iden3/go-iden3-crypto/poseidon"
	mpt "github.com/keybase/go-merkle-tree"
)

type valueFactory struct{}

func (valueFactory) Construct() interface{} {
	return struct {
		Value []byte
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

	if hash == nil {
		return common.Hash{}, nil
	}

	return common.Hash(hash[:]), nil
}

func (tree *MerkleTree) Find(key common.Hash) ([]byte, error) {
	val, _, err := tree.tree.Find(context.TODO(), mpt.Hash(key[:]))

	if err == NodeLookupFailed {
		return nil, nil
	}
	if val == nil {
		return nil, nil
	}

	val_full, ok := val.(struct {Value []byte})

	if !ok {
		return nil, fmt.Errorf("Failed to decode tree node %s", val)
	}

	return val_full.Value, nil
}

func (tree *MerkleTree) Upsert(key common.Hash, value []byte) error {
	return tree.tree.Upsert(context.TODO(), mpt.KeyValuePair{Key: key[:], Value: struct {Value []byte } {Value: value}}, nil)
}

func GetMerkleTree(tx Tx, root common.Hash) *MerkleTree {
	cfg := mpt.NewConfig(poseidonHasher{}, mpt.ChildIndex(4), mpt.ChildIndex(1), valueFactory{})
	eng := GetEngine(tx, root)
	return &MerkleTree{tree: mpt.NewTree(eng, cfg), eng: eng}
}
