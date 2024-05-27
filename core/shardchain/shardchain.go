package shardchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/features"
	"github.com/rs/zerolog"
)

type BlockGenerator interface {
	GenerateBlock(ctx context.Context, msgs []*types.Message) (*types.Block, error)
}

type ShardChain struct {
	Id types.ShardId
	db db.DB

	logger *zerolog.Logger
	timer  common.Timer

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

func (c *ShardChain) HandleDeployMessage(message *types.Message, index uint64, es *execution.ExecutionState) error {
	return es.HandleDeployMessage(message, index)
}

func (c *ShardChain) HandleExecutionMessage(message *types.Message, index uint64, interpreter *vm.EVMInterpreter, es *execution.ExecutionState) error {
	addr := message.To
	c.logger.Debug().Msgf("Call contract %s", addr)

	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &message.Value.Int, gas)

	accountState := es.GetAccount(addr)
	contract.Code = accountState.Code

	_, err := interpreter.Run(contract, message.Data, false)
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

func (c *ShardChain) validateMessage(es *execution.ExecutionState, message *types.Message, index uint64) (bool, error) {
	if !features.EnableSignatureCheck {
		return true, nil
	}
	addr := message.From
	accountState := es.GetAccount(addr)

	r := &types.Receipt{
		Success:         false,
		GasUsed:         0,
		MsgHash:         es.MessageHash,
		MsgIndex:        index,
		ContractAddress: addr,
	}
	if accountState == nil {
		r.Logs = es.Logs[es.MessageHash]
		es.AddReceipt(r)
		c.logger.Debug().Stringer("address", addr).Msg("Invalid address")
		return false, nil
	}

	if len(accountState.PublicKey) != 0 {
		ok, err := message.ValidateSignature(accountState.PublicKey)
		if err != nil {
			return false, err
		}
		if !ok {
			r.Logs = es.Logs[es.MessageHash]
			es.AddReceipt(r)
			c.logger.Debug().Stringer("address", addr).Msg("Invalid signature")
			return false, nil
		}
	}

	if accountState.Seqno != message.Seqno {
		r.Logs = es.Logs[es.MessageHash]
		es.AddReceipt(r)
		c.logger.Debug().
			Stringer("address", addr).
			Uint64("account.seqno", accountState.Seqno).
			Uint64("message.seqno", message.Seqno).
			Msg("Seqno gap")
		return false, nil
	}

	return true, nil
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
	defer roTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, c.Id, c.timer)
	if err != nil {
		return nil, err
	}

	for _, message := range msgs {
		msgHash := message.Hash()
		index := es.AddMessage(message)
		es.MessageHash = msgHash

		ok, err := c.validateMessage(es, message, index)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		evm := vm.EVM{
			StateDB: es,
		}
		interpreter := vm.NewEVMInterpreter(&evm)

		// Deploy message
		if message.To.IsEmpty() {
			if err := c.HandleDeployMessage(message, index, es); err != nil {
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
			es.SetShardHash(types.ShardId(i), lastBlockHash)
		}
	} else {
		lastBlockHash, err := c.getHashLastBlock(roTx, types.MasterShardId)
		if err != nil {
			return nil, err
		}
		es.SetMasterchainHash(lastBlockHash)
	}

	blockId := types.BlockNumber(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(rwTx, c.Id, es.PrevBlock).Id + 1
	}

	blockHash, err := es.Commit(blockId)
	if err != nil {
		return nil, err
	}

	block, err := execution.PostprocessBlock(rwTx, c.Id, blockHash)
	if err != nil {
		return nil, err
	}

	if err = rwTx.Commit(); err != nil {
		return nil, err
	}

	return block, nil
}

func NewShardChain(
	shardId types.ShardId,
	db db.DB,
	nShards int,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId), false /* noColor */)
	timer := common.NewTimer()
	return &ShardChain{Id: shardId, db: db, logger: logger, timer: timer, nShards: nShards}
}
