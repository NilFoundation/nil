package shardchain

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

type BlockGenerator interface {
	GenerateBlock(ctx context.Context, msgs []*types.Message) (*types.Block, error)
}

type ShardChain struct {
	Id types.ShardId
	db db.DB

	logger *zerolog.Logger

	nShards int
}

var _ BlockGenerator = new(ShardChain)

func (c *ShardChain) isMasterchain() bool {
	return c.Id == types.MasterShardId
}

func (c *ShardChain) getHashLastBlock(roTx db.Tx, shardId types.ShardId) (common.Hash, error) {
	lastBlockRaw, err := roTx.Get(db.LastBlockTable, shardId.Bytes())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, fmt.Errorf("failed getting last block %w for shard %d", err, shardId)
	}
	lastBlockHash := common.EmptyHash
	if lastBlockRaw != nil {
		lastBlockHash = common.Hash(*lastBlockRaw)
	}
	return lastBlockHash, nil
}

func (c *ShardChain) HandleDeployMessage(message *types.Message, index uint64, interpreter *vm.EVMInterpreter, es *execution.ExecutionState) error {
	addr := execution.CreateAddress(message.From, message.Seqno)
	c.logger.Debug().Msgf("Create new contract %s", addr)

	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &message.Value.Int, gas)
	contract.Code = message.Data

	code, err := interpreter.Run(contract, nil, false)
	if err != nil {
		c.logger.Error().Msg("message failed")
		return err
	}
	if err := es.HandleDeployMessage(message, code, index); err != nil {
		return err
	}
	return nil
}

func (c *ShardChain) HandleExecutionMessage(message *types.Message, index uint64, interpreter *vm.EVMInterpreter, es *execution.ExecutionState) error {
	addr := message.To
	c.logger.Debug().Msgf("Call contract %s", addr)

	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &message.Value.Int, gas)

	accountState := es.GetAccount(addr)
	contract.Code = accountState.Code

	_, err := interpreter.Run(contract, nil, false)
	if err != nil {
		c.logger.Error().Msg("message failed")
		return err
	}
	r := types.Receipt{
		Success:         true,
		GasUsed:         uint32(gas - contract.Gas),
		Logs:            es.Logs[es.MessageHash],
		MsgHash:         es.MessageHash,
		MsgIndex:        index,
		ContractAddress: addr,
	}
	es.AddReceipt(&r)
	return nil
}

func (c *ShardChain) GenerateBlock(ctx context.Context, msgs []*types.Message) (*types.Block, error) {
	rwTx, err := c.db.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer rwTx.Rollback()
	roTx, err := c.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}

	lastBlockHashBytes, err := rwTx.Get(db.LastBlockTable, c.Id.Bytes())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed getting last block: %w", err)
	}

	lastBlockHash := common.EmptyHash
	// No previous blocks yet
	if lastBlockHashBytes != nil {
		lastBlockHash = common.Hash(*lastBlockHashBytes)
	}

	es, err := execution.NewExecutionState(rwTx, c.Id, lastBlockHash)
	if err != nil {
		return nil, err
	}

	for _, message := range msgs {
		index := es.AddMessage(message)

		evm := vm.EVM{
			StateDB: es,
		}
		interpreter := vm.NewEVMInterpreter(&evm)

		// Deploy message
		if bytes.Equal(message.To[:], common.EmptyAddress[:]) {
			if err := c.HandleDeployMessage(message, index, interpreter, es); err != nil {
				return nil, err
			}
		} else {
			if err := c.HandleExecutionMessage(message, index, interpreter, es); err != nil {
				return nil, err
			}
		}
	}

	if c.isMasterchain() {
		for i := 1; i < c.nShards; i++ {
			lastBlockHash, err := c.getHashLastBlock(roTx, types.ShardId(i))
			if err != nil {
				return nil, err
			}
			es.SetShardHash(uint64(i), lastBlockHash)
		}
	} else {
		lastBlockHash, err := c.getHashLastBlock(roTx, types.MasterShardId)
		if err != nil {
			return nil, err
		}
		es.SetMasterchainHash(lastBlockHash)
	}

	blockId := uint64(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(rwTx, c.Id, es.PrevBlock).Id + 1
	}

	blockHash, err := es.Commit(blockId)
	if err != nil {
		return nil, err
	}

	if err = rwTx.Put(db.LastBlockTable, c.Id.Bytes(), blockHash[:]); err != nil {
		return nil, err
	}

	block := db.ReadBlock(rwTx, c.Id, blockHash)

	if err = rwTx.Commit(); err != nil {
		return nil, err
	}

	return block, nil
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

	lastBlockHashBytes, err := rwTx.Get(db.LastBlockTable, c.Id.Bytes())

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

	addr := common.BytesToAddress([]byte("contract-" + c.Id.String()))

	accountState := es.GetAccount(addr)

	value := uint256.Int{}
	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &value, gas)

	evm := vm.EVM{
		StateDB: es,
	}
	interpreter := vm.NewEVMInterpreter(&evm)

	initialGas := contract.Gas

	if accountState == nil {
		c.logger.Debug().Msgf("Create new contract %s", addr)

		// constructor for a simple counter contract
		contract.Code = hexutil.FromHex("6009600c60003960096000f3600054600101600055")
		code, err := interpreter.Run(contract, nil, false)
		if err != nil {
			c.logger.Error().Msg("transaction failed")
			return common.EmptyHash, err
		}

		if err = es.CreateAccount(addr); err != nil {
			return common.EmptyHash, err
		}
		es.SetCode(addr, code)
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
		for i := range c.nShards - 1 {
			lastBlockHash, err := c.getHashLastBlock(roTx, types.ShardId(i+1))
			if err != nil {
				return common.EmptyHash, err
			}
			es.SetShardHash(uint64(i), lastBlockHash)
		}
	} else {
		lastBlockHash, err := c.getHashLastBlock(roTx, types.MasterShardId)
		if err != nil {
			return common.EmptyHash, err
		}
		es.SetMasterchainHash(lastBlockHash)
	}

	blockId := uint64(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(rwTx, c.Id, es.PrevBlock).Id + 1
	}

	// Create receipt for the executed message
	receipt := types.Receipt{
		Success:         true,
		GasUsed:         uint32(initialGas - contract.Gas),
		Logs:            es.Logs[es.MessageHash],
		ContractAddress: addr,
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{&receipt})
	es.Receipts = append(es.Receipts, &receipt)

	blockHash, err := es.Commit(blockId)
	if err != nil {
		return common.EmptyHash, err
	}

	if err = rwTx.Put(db.LastBlockTable, c.Id.Bytes(), blockHash[:]); err != nil {
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
	shardId types.ShardId,
	db db.DB,
	nShards int,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	return &ShardChain{shardId, db, logger, nShards}
}
