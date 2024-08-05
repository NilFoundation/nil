package mpt

import (
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Getter interface {
	Get(key []byte) ([]byte, error)
}

type Setter interface {
	Set(key, value []byte) error
}

type DbGetter struct {
	tx        db.RoTx
	shardId   types.ShardId
	tableName db.ShardedTableName
}

func NewDbGetter(tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName) *DbGetter {
	return &DbGetter{tx, shardId, tableName}
}

func (g *DbGetter) Get(key []byte) ([]byte, error) {
	return g.tx.GetFromShard(g.shardId, g.tableName, key)
}

type DbSetter struct {
	tx        db.RwTx
	shardId   types.ShardId
	tableName db.ShardedTableName
}

func NewDbSetter(tx db.RwTx, shardId types.ShardId, tableName db.ShardedTableName) *DbSetter {
	return &DbSetter{tx, shardId, tableName}
}

func (s *DbSetter) Set(key, value []byte) error {
	return s.tx.PutToShard(s.shardId, s.tableName, key, value)
}

type MapGetter struct {
	m map[string][]byte
}

func NewMapGetter(m map[string][]byte) *MapGetter {
	return &MapGetter{m}
}

func (g *MapGetter) Get(key []byte) ([]byte, error) {
	return g.m[string(key)], nil
}

type MapSetter struct {
	m map[string][]byte
}

func NewMapSetter(m map[string][]byte) *MapSetter {
	return &MapSetter{m}
}

func (s *MapSetter) Set(key, value []byte) error {
	s.m[string(key)] = value
	return nil
}
