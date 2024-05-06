package shardchain

import (
	"fmt"
	"log"
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

	last_block_hash_bytes, err := tx.Get(db.LastBlockTable, []byte(strconv.Itoa(c.Id)))

	if err != nil {
		return common.EmptyHash, err
	}

	last_block_hash := common.Hash{}
	// No previous blocks yet
	if last_block_hash_bytes != nil {
		last_block_hash = common.Hash(*last_block_hash_bytes)
	}

	es, err := execution.NewExecutionState(tx, last_block_hash)

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

		err = es.CreateContract(addr, code)

		if err != nil {
			return common.EmptyHash, err
		}
	} else {
		c.logger.Debug().Msgf("Update storage of contract %s", addr)
		storage_key := common.BytesToHash([]byte("storage-key"))
		val, err := es.GetState(addr, storage_key)

		if err != nil {
			return common.EmptyHash, err
		}

		val.AddUint64(&val, 1)

		err = es.SetState(addr, storage_key, val)

		if err != nil {
			return common.EmptyHash, err
		}
	}

	block_hash, err := es.Commit()

	if err != nil {
		return common.EmptyHash, err
	}

	if err = tx.Put(db.LastBlockTable, []byte(strconv.Itoa(c.Id)), block_hash[:]); err != nil {
		return common.EmptyHash, err
	}

	if err = tx.Commit(); err != nil {
		return common.EmptyHash, err
	}

	return block_hash, nil
}

func (c *ShardChain) Collate(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	c.logger.Info().Msg("running shardchain")

	block_hash, err := c.testTransaction(ctx)
	if err != nil {
		log.Fatal(err)
	}

	c.logger.Debug().Msgf("new block : %+v", block_hash)

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
