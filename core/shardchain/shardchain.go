package shardchain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/features"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type BlockGenerator interface {
	GenerateBlock(ctx context.Context, msgs []*types.Message) (*types.Block, error)
	GenerateZerostate(ctx context.Context) error
}

type ShardChain struct {
	Id types.ShardId
	db db.DB

	logger *zerolog.Logger
	timer  common.Timer

	nShards int
}

var _ BlockGenerator = new(ShardChain)

var MainPrivateKey *ecdsa.PrivateKey

func init() {
	// All this info should be provided via zerostate / config / etc
	// but for now it's hardcoded for simplicity.
	pubkeyHex := "02eb7216201e65f0a41bc655ada025ad943b79d38aca4d671cbd9875b9604f1ac1"
	pubkey, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to prepare main key (decode hex)")
	}

	key, err := crypto.DecompressPubkey(pubkey)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to prepare main key (unmarshal)")
	}

	keyD := new(big.Int)
	keyD.SetString("29471664811761943693235393363502564971627872515497410365595228231506458150155", 10)
	MainPrivateKey = &ecdsa.PrivateKey{PublicKey: *key, D: keyD}

	if !key.Equal(MainPrivateKey.Public()) {
		log.Fatal().Msg("Consistency check on key recover failed")
	}
}

func (c *ShardChain) isMasterchain() bool {
	return c.Id == types.MasterShardId
}

func (c *ShardChain) HandleDeployMessage(message *types.Message, index uint64, es *execution.ExecutionState) error {
	return es.HandleDeployMessage(message, index)
}

func (c *ShardChain) HandleExecutionMessage(message *types.Message, index uint64, interpreter *vm.EVMInterpreter, es *execution.ExecutionState) error {
	addr := message.To
	c.logger.Debug().Msgf("Call contract %s", addr)

	// TODO: use gas from message
	gas := uint64(1000000)
	contract := vm.NewContract((vm.AccountRef)(addr), (vm.AccountRef)(addr), &message.Value.Int, gas)

	accountState := es.GetAccount(addr)
	contract.Code = accountState.Code

	// TODO: not ignore result here
	_, err := interpreter.Run(contract, message.Data, false)
	if err != nil {
		c.logger.Error().Msg("execution message failed")
		return err
	}
	r := types.Receipt{
		Success:         true,
		GasUsed:         uint32(gas - contract.Gas),
		Logs:            es.Logs[es.InMessageHash],
		MsgHash:         es.InMessageHash,
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
		MsgHash:         es.InMessageHash,
		MsgIndex:        index,
		ContractAddress: addr,
	}
	if accountState == nil {
		r.Logs = es.Logs[es.InMessageHash]
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
			r.Logs = es.Logs[es.InMessageHash]
			es.AddReceipt(r)
			c.logger.Debug().Stringer("address", addr).Msg("Invalid signature")
			return false, nil
		}
	}

	if accountState.Seqno != message.Seqno {
		r.Logs = es.Logs[es.InMessageHash]
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

func (c *ShardChain) setLastBlockHashes(tx db.RoTx, es *execution.ExecutionState) error {
	if c.isMasterchain() {
		for i := 1; i < c.nShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
			if err != nil {
				return err
			}
			es.SetShardHash(shardId, lastBlockHash)
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(tx, types.MasterShardId)
		if err != nil {
			return err
		}
		es.SetMasterchainHash(lastBlockHash)
	}
	return nil
}

func (c *ShardChain) GenerateZerostate(ctx context.Context) error {
	roTx, err := c.db.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer roTx.Rollback()

	lastBlockHash, err := db.ReadLastBlockHash(roTx, c.Id)
	if err != nil {
		c.logger.Fatal().Err(err).Msg("Failed to get the latest block")
	}

	if lastBlockHash != common.EmptyHash {
		return nil
	}

	rwTx, err := c.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer rwTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, c.Id, c.timer)
	if err != nil {
		return err
	}

	mainDeployMsg := &types.DeployMessage{
		ShardId:   uint32(c.Id),
		Seqno:     0,
		PublicKey: crypto.CompressPubkey(&MainPrivateKey.PublicKey),
	}

	pub := crypto.CompressPubkey(&MainPrivateKey.PublicKey)
	addr := common.PubkeyBytesToAddress(uint32(c.Id), pub)
	es.CreateAccount(addr)
	es.CreateContract(addr)
	es.SetInitState(addr, mainDeployMsg)

	mainBalance, err := uint256.FromDecimal("1000000000000")
	if err != nil {
		return err
	}

	es.SetBalance(addr, *mainBalance)

	if err := c.setLastBlockHashes(roTx, es); err != nil {
		return err
	}

	blockId := types.BlockNumber(0)
	blockHash, err := es.Commit(blockId)
	if err != nil {
		return err
	}

	_, err = execution.PostprocessBlock(rwTx, c.Id, blockHash)
	if err != nil {
		return err
	}

	if err = rwTx.Commit(); err != nil {
		return err
	}

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
	defer roTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, c.Id, c.timer)
	if err != nil {
		return nil, err
	}

	for _, message := range msgs {
		msgHash := message.Hash()
		index := es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := c.validateMessage(es, message, index)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		accountState := es.GetAccount(message.From)
		accountState.SetSeqno(accountState.Seqno + 1)

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

	if err := c.setLastBlockHashes(roTx, es); err != nil {
		return nil, err
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
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId))
	timer := common.NewTimer()
	return &ShardChain{Id: shardId, db: db, logger: logger, timer: timer, nShards: nShards}
}
