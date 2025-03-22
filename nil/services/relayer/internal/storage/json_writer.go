package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
)

type jsonDbWriter[T any] struct {
	table   db.TableName
	storage *BaseStorage
	upsert  bool
}

func NewJSONWriter[T any](tableName db.TableName, storage *BaseStorage, upsert bool) *jsonDbWriter[T] {
	return &jsonDbWriter[T]{
		table:   tableName,
		storage: storage,
		upsert:  upsert,
	}
}

func (jdwr *jsonDbWriter[T]) PutTx(ctx context.Context, key []byte, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSerializationFailed, err)
	}

	tx, err := jdwr.storage.Database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if !jdwr.upsert {
		exists, err := tx.Exists(jdwr.table, key)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w: table=%s key=%v", ErrKeyExists, jdwr.table, key)
		}
	}

	if err := tx.Put(jdwr.table, key, data); err != nil {
		return err
	}

	return jdwr.storage.Commit(tx)
}

func (jdwr *jsonDbWriter[T]) PutManyTx(ctx context.Context, reqs []InsertRequest[T]) error {
	tx, err := jdwr.storage.Database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, req := range reqs {
		data, err := json.Marshal(req.Value)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSerializationFailed, err)
		}

		if !jdwr.upsert {
			exists, err := tx.Exists(jdwr.table, req.Key)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%w: table=%s key=%v", ErrKeyExists, jdwr.table, req.Key)
			}
		}
		if err := tx.Put(jdwr.table, req.Key, data); err != nil {
			return err
		}
	}

	return jdwr.storage.Commit(tx)
}

type InsertRequest[T any] struct {
	Key   []byte
	Value T
}

func MakeInsertRequests[E ~[]T, T any](in E, keyFunc func(T) []byte) []InsertRequest[T] {
	ret := make([]InsertRequest[T], len(in))
	for i, v := range in {
		ret[i] = InsertRequest[T]{
			Key:   keyFunc(v),
			Value: v,
		}
	}
	return ret
}
