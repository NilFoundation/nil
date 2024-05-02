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
}

var _ mpt.StorageEngine = (*tableClient)(nil)

var NodeLookupFailed = errors.New("Node lookup failed")

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
		return []byte{}, NodeLookupFailed
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
	mpt_hash := mpt.Hash(root[:])
	empty_hash := common.Hash{}
	if root == empty_hash {
		mpt_hash = nil
	}

	return &tableClient{tx: tx, root: mpt_hash}
}
