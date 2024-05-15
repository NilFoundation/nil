package shardchain

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

	// todo: can be probably moved out (and the Transaction type removed completely since we have Message type now)
	pool []Transaction

	logger *zerolog.Logger

	NShards int // todo: used for test transaction only, remove in the future
}

func (c *ShardChain) isMasterchain() bool {
	return c.Id == 0
}

func (c *ShardChain) getHashLastBlock(roTx db.Tx, shardId uint64) (common.Hash, error) {
	lastBlockRaw, err := roTx.Get(db.LastBlockTable, []byte(strconv.FormatUint(shardId, 10)))
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, fmt.Errorf("failed getting last block %w for shard %d", err, shardId)
	}
	lastBlockHash := common.EmptyHash
	if lastBlockRaw != nil {
		lastBlockHash = common.Hash(*lastBlockRaw)
	}
	return lastBlockHash, nil
}

func (c *ShardChain) testTransaction(ctx context.Context) (common.Hash, error) {
	rwTx, err := c.db.CreateRwTx(ctx)
	if err != nil {
		return common.EmptyHash, err
	}
	defer rwTx.Rollback()
	roTx, err := c.db.CreateRoTx(ctx)
	if err != nil {
		return common.EmptyHash, err
	}

	lastBlockHashBytes, err := rwTx.Get(db.LastBlockTable, []byte(strconv.Itoa(c.Id)))

	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, fmt.Errorf("failed getting last block: %w", err)
	}

	lastBlockHash := common.EmptyHash
	// No previous blocks yet
	if lastBlockHashBytes != nil {
		lastBlockHash = common.Hash(*lastBlockHashBytes)
	}

	es, err := execution.NewExecutionState(rwTx, c.Id, lastBlockHash)

	if err != nil {
		return common.EmptyHash, err
	}

	addr := common.BytesToAddress([]byte("contract-" + strconv.Itoa(c.Id)))

	accountState := es.GetAccount(addr)

	value := uint256.Int{}
	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &value, gas)

	evm := vm.EVM{
		StateDB: es,
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

	if c.isMasterchain() {
		for i := 1; i < c.NShards; i++ {
			lastBlockHash, err := c.getHashLastBlock(roTx, uint64(i))
			if err != nil {
				return common.EmptyHash, err
			}
			es.SetShardHash(uint64(i), lastBlockHash)
		}
	} else {
		lastBlockHash, err := c.getHashLastBlock(roTx, uint64(0))
		if err != nil {
			return common.EmptyHash, err
		}
		es.SetMasterchainHash(lastBlockHash)
	}

	blockId := uint64(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(rwTx, es.PrevBlock).Id + 1
	}

	blockHash, err := es.Commit(blockId)

	if err != nil {
		return common.EmptyHash, err
	}

	if err = rwTx.Put(db.LastBlockTable, []byte(strconv.Itoa(c.Id)), blockHash[:]); err != nil {
		return common.EmptyHash, err
	}

	if err = rwTx.Commit(); err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}

func (c *ShardChain) Collate(ctx context.Context) error {
	c.logger.Info().Msg("running shardchain")

	blockHash, err := c.testTransaction(ctx)
	if err != nil {
		return err
	}

	c.logger.Debug().Msgf("new block : %+v", blockHash)
	return nil
}

func NewShardChain(
	shardId int,
	db db.DB,
	nShards int,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	return &ShardChain{shardId, db, nil, logger, nShards}
}
