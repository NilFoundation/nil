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

func (c *ShardChain) TestTransaction() (common.Hash, error) {
	tx, err := c.db.CreateTx(context.TODO())
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

	contract_exists, err := es.ContractExists(addr)

	if err != nil {
		return common.EmptyHash, err
	}

	if (!contract_exists) {
		c.logger.Debug().Msgf("Create new contract %s", addr)
		code := []byte("asdf")

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

	err = tx.Put(db.LastBlockTable, []byte(strconv.Itoa(c.Id)), block_hash[:])

	if err != nil {
		return common.EmptyHash, err
	}

	err = tx.Commit()

	if err != nil {
		return common.EmptyHash, err
	}

	return block_hash, nil
}

func (c *ShardChain) Collate(wg *sync.WaitGroup) {
	defer wg.Done()

	c.logger.Info().Msg("running shardchain")

	block_hash, err := c.TestTransaction()
	if err != nil {
		log.Fatal(err)
	}
	//tree := db.GetMerkleTree(fmt.Sprintf("shard-%d-smart-contracts", c.Id), c.dbClient)
	// genesisBlock := &types.Block{SmartContracts: tree}

	// nextBlk := genBlock(c.dbClient, genesisBlock, "contract-addr-to-update")
	// genBlock(c.dbClient, nextBlk, "contract-addr-to-update")

	c.logger.Debug().Msgf("new block : %+v", block_hash)

	evm := vm.NewEVMInterpreter(nil)
	for _, tx := range c.pool {
		if _, err := evm.Run(&vm.Contract{}, tx.calldata, false); err != nil {
			logger.Error().Msg("transaction failed")
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
