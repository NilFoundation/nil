package db

import (
	"context"

	"github.com/iden3/go-iden3-crypto/poseidon"
	mpt "github.com/keybase/go-merkle-tree"
)

type TreeWrapper struct {
	Tree   *mpt.Tree
	Engine mpt.StorageEngine
}

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

func GetMerkleTree(table string, client *DBClient) *TreeWrapper {
	cfg := mpt.NewConfig(poseidonHasher{}, mpt.ChildIndex(4), mpt.ChildIndex(1), valueFactory{})
	eng := client.GetEngine(table)
	return &TreeWrapper{mpt.NewTree(eng, cfg), eng}
}
