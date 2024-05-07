package shardchain

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/rs/zerolog"
)

type Transaction struct {
	//address  common.Address
	calldata []byte
}

type ShardChain struct {
	Id int
	db db.DB

	pool []Transaction

	logger *zerolog.Logger
}

func (c *ShardChain) testTransaction(ctx context.Context) (common.Hash, error) {
	tx, err := c.db.CreateTx(ctx)
	if err != nil {
		return common.EmptyHash, err
	}
	defer tx.Rollback()

	lastBlockHashBytes, err := tx.Get(db.LastBlockTable, []byte(strconv.Itoa(c.Id)))

	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, fmt.Errorf("failed getting last block: %w", err)
	}

	lastBlockHash := common.EmptyHash
	// No previous blocks yet
	if lastBlockHashBytes != nil {
		lastBlockHash = common.Hash(*lastBlockHashBytes)
	}

	es, err := execution.NewExecutionState(tx, lastBlockHash)

	if err != nil {
		return common.EmptyHash, err
	}

	addr := common.BytesToHash([]byte("contract-" + strconv.Itoa(c.Id)))

	contractExists, err := es.ContractExists(addr)

	if err != nil {
		return common.EmptyHash, err
	}

	if !contractExists {
		c.logger.Debug().Msgf("Create new contract %s", addr)
		code := []byte("Real code should eventually be here. Now it's just a stub.")

		if err = es.CreateContract(addr, code); err != nil {
			return common.EmptyHash, err
		}
	} else {
		c.logger.Debug().Msgf("Update storage of contract %s", addr)
		storageKey := common.BytesToHash([]byte("storage-key"))
		val, err := es.GetState(addr, storageKey)

		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return common.EmptyHash, err
		}

		val.AddUint64(&val, 1)

		if err = es.SetState(addr, storageKey, val); err != nil {
			return common.EmptyHash, err
		}
	}

	blockHash, err := es.Commit()

	if err != nil {
		return common.EmptyHash, err
	}

	if err = tx.Put(db.LastBlockTable, []byte(strconv.Itoa(c.Id)), blockHash[:]); err != nil {
		return common.EmptyHash, err
	}

	if err = tx.Commit(); err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}

func (c *ShardChain) Collate(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	c.logger.Info().Msg("running shardchain")

	blockHash, err := c.testTransaction(ctx)
	if err != nil {
		c.logger.Fatal().Msgf("collation failed: %s", err.Error())
	}

	c.logger.Debug().Msgf("new block : %+v", blockHash)

	evm := vm.NewEVMInterpreter(nil)
	for _, tx := range c.pool {
		if _, err := evm.Run(&vm.Contract{}, tx.calldata, false); err != nil {
			c.logger.Error().Msg("transaction failed")
		}
	}
}

func NewShardChain(
	shardId int,
	db db.DB,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	return &ShardChain{shardId, db, nil, logger}
}
