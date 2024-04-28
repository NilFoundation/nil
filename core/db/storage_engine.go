package db

import (
	"context"
	"encoding/hex"

	mpt "github.com/keybase/go-merkle-tree"
)

type DBClient struct {
	engines map[string]mpt.StorageEngine
}

type tableClient struct {
	root   mpt.Hash
	leaves map[string][]byte
}

var _ mpt.StorageEngine = (*tableClient)(nil)

func (c *tableClient) CommitRoot(_ context.Context, prev mpt.Hash, curr mpt.Hash, txinfo mpt.TxInfo) error {
	c.root = curr
	return nil
}

func (c *tableClient) LookupNode(_ context.Context, h mpt.Hash) (b []byte, err error) {
	return c.leaves[hex.EncodeToString(h)], nil
}

func (c *tableClient) LookupRoot(_ context.Context) (mpt.Hash, error) {
	return c.root, nil
}

func (c *tableClient) StoreNode(_ context.Context, key mpt.Hash, b []byte) error {
	c.leaves[hex.EncodeToString(key)] = b
	return nil
}

func newDBTableStorageEngine(table string, client *DBClient) mpt.StorageEngine {
	client.engines[table] = &tableClient{leaves: make(map[string][]byte)}
	return client.engines[table]
}

func NewDBClient() *DBClient {
	return &DBClient{engines: make(map[string]mpt.StorageEngine)}
}

func (c *DBClient) GetEngine(table string) mpt.StorageEngine {
	if e, ok := c.engines[table]; !ok {
		return newDBTableStorageEngine(table, c)
	} else {
		return e
	}
}
