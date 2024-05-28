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
	GenerateZeroState(ctx context.Context, es *execution.ExecutionState) error
	CreateRoTx(ctx context.Context) (db.RoTx, error)
	CreateRwTx(ctx context.Context) (db.RwTx, error)
	HandleMessages(ctx context.Context, es *execution.ExecutionState, msgs []*types.Message) error
}

type ShardChain struct {
	Id types.ShardId
	db db.DB

	logger *zerolog.Logger
	timer  common.Timer
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

func (c *ShardChain) GenerateZeroState(ctx context.Context, es *execution.ExecutionState) error {
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

	return nil
}

func (c *ShardChain) CreateRoTx(ctx context.Context) (db.RoTx, error) {
	return c.db.CreateRoTx(ctx)
}

func (c *ShardChain) CreateRwTx(ctx context.Context) (db.RwTx, error) {
	return c.db.CreateRwTx(ctx)
}

func (c *ShardChain) HandleMessages(ctx context.Context, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		index := es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := c.validateMessage(es, message, index)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		evm := vm.NewEVM(blockContext, es)
		interpreter := evm.Interpreter()

		// Deploy message
		if message.To.IsEmpty() {
			if err := es.HandleDeployMessage(message, index); err != nil {
				return err
			}
		} else {
			if err := es.HandleExecutionMessage(message, index, interpreter); err != nil {
				return err
			}
		}
	}

	return nil
}

func NewShardChain(
	shardId types.ShardId,
	db db.DB,
) *ShardChain {
	logger := common.NewLogger(fmt.Sprintf("shard-%d", shardId))
	timer := common.NewTimer()
	return &ShardChain{Id: shardId, db: db, logger: logger, timer: timer}
}
