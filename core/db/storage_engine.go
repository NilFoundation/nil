package db

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/common"
	mpt "github.com/keybase/go-merkle-tree"
)

type tableClient struct {
	root  mpt.Hash
	tx    Tx
	table string
}

var _ mpt.StorageEngine = (*tableClient)(nil)

func (c *tableClient) CommitRoot(_ context.Context, prev mpt.Hash, curr mpt.Hash, txinfo mpt.TxInfo) error {
	c.root = curr
	return nil
}

func (c *tableClient) LookupNode(_ context.Context, h mpt.Hash) (b []byte, err error) {
	node, err := c.tx.Get(MptTable, h[:])
	if err != nil {
		return []byte{}, err
	}
	if node == nil {
		return []byte{}, errors.New("Node lookup failed")
	}

	return *node, nil
}

func (c *tableClient) LookupRoot(_ context.Context) (mpt.Hash, error) {
	return c.root, nil
}

func (c *tableClient) StoreNode(_ context.Context, key mpt.Hash, b []byte) error {
	return c.tx.Put(MptTable, key[:], b)
}

func GetEngine(tx Tx, root common.Hash) mpt.StorageEngine {
	return &tableClient{tx: tx, root: mpt.Hash(root[:])}
}
