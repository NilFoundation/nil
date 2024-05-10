package shardchain

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

type Transaction struct {
	//address  common.Address
	//calldata []byte
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

	addr := common.BytesToAddress([]byte("contract-" + strconv.Itoa(c.Id)))

	accountState, err := es.GetAccount(addr)

	if err != nil {
		return common.EmptyHash, err
	}

	value := uint256.Int{}
	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &value, gas)

	evm := vm.EVM{
		StateDB: vm.NewStateDB(es),
	}
	interpreter := vm.NewEVMInterpreter(&evm)

	if accountState == nil {
		c.logger.Debug().Msgf("Create new contract %s", addr)

		// constructor for a simple counter contract
		contract.Code = hexutil.FromHex("6009600c60003960096000f3600054600101600055")
		code, err := interpreter.Run(contract, nil, false)
		if err != nil {
			c.logger.Error().Msg("transaction failed")
			return common.EmptyHash, err
		}

		if err = es.CreateContract(addr, code); err != nil {
			return common.EmptyHash, err
		}
	} else {
		contract.Code = accountState.Code
		c.logger.Debug().Msgf("Update storage of contract %s", addr)

		_, err := interpreter.Run(contract, nil, false)
		if err != nil {
			c.logger.Error().Msg("transaction failed")
			return common.EmptyHash, err
		}

		number := evm.StateDB.GetState(addr, common.EmptyHash)
		c.logger.Debug().Msgf("Contract storage is now %v", number)
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

}

func NewShardChain(
	shardId int,
	db db.DB,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	return &ShardChain{shardId, db, nil, logger}
}
