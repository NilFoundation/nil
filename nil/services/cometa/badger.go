package cometa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/badger/v4"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type StorageBadger struct {
	db *badger.DB
}

var _ Storage = new(StorageBadger)

func NewStorageBadger(cfg *Config) (*StorageBadger, error) {
	opts := badger.DefaultOptions(cfg.DbPath).WithLogger(nil)
	badgerInstance, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	storage := &StorageBadger{
		db: badgerInstance,
	}

	return storage, nil
}

func (s *StorageBadger) StoreContract(ctx context.Context, contractData *ContractData, address types.Address) error {
	tx := s.createRwTx()
	defer tx.Discard()

	data, err := json.Marshal(contractData)
	if err != nil {
		return err
	}

	if err = tx.Set(address.Bytes(), data); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *StorageBadger) LoadContractData(ctx context.Context, address types.Address) (*ContractData, error) {
	tx := s.createRoTx()
	defer tx.Discard()

	item, err := tx.Get(address.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get contract data: %w", err)
	}
	data, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to copy value: %w", err)
	}

	res := new(ContractData)
	if err = json.Unmarshal(data, res); err != nil {
		return nil, err
	}

	return res, nil
}

func (s *StorageBadger) GetAbi(ctx context.Context, address types.Address) (string, error) {
	return "", errors.New("not implemented")
}

func (s *StorageBadger) createRoTx() *badger.Txn {
	return s.db.NewTransaction(false)
}

func (s *StorageBadger) createRwTx() *badger.Txn {
	return s.db.NewTransaction(true)
}
