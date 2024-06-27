package execution

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

type BlockGeneratorParams struct {
	ShardId       types.ShardId
	NShards       int
	TraceEVM      bool
	Timer         common.Timer
	GasBasePrice  *uint256.Int
	GasPriceScale float64
}

func NewBlockGeneratorParams(shardId types.ShardId, nShards int, gasBasePrice *uint256.Int, gasPriceScale float64) BlockGeneratorParams {
	return BlockGeneratorParams{
		ShardId:       shardId,
		NShards:       nShards,
		Timer:         common.NewTimer(),
		GasBasePrice:  gasBasePrice,
		GasPriceScale: gasPriceScale,
	}
}

type BlockGenerator struct {
	params BlockGeneratorParams

	txOwner        *TxOwner
	executionState *ExecutionState

	logger zerolog.Logger
}

func NewBlockGenerator(params BlockGeneratorParams, txOwner *TxOwner) (*BlockGenerator, error) {
	executionState, err := NewExecutionStateForShard(txOwner.RwTx, params.ShardId, params.Timer)
	if err != nil {
		return nil, err
	}
	executionState.TraceVm = params.TraceEVM
	return &BlockGenerator{
		params:         params,
		txOwner:        txOwner,
		executionState: executionState,
		logger: logging.NewLogger("block-gen").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
	}, nil
}

func (g *BlockGenerator) GenerateZeroState(zeroState string) error {
	g.logger.Info().Msg("Generating zero-state...")

	if err := g.executionState.GenerateZeroState(zeroState); err != nil {
		return err
	}

	_, err := g.finalize()
	return err
}

func (g *BlockGenerator) GenerateBlock(inMsgs, outMsgs []*types.Message, defaultGasPrice *uint256.Int) (*types.Block, error) {
	gasPrice, err := db.ReadGasPerShard(g.txOwner.RoTx, g.executionState.ShardId)
	if errors.Is(err, db.ErrKeyNotFound) {
		gasPrice = *defaultGasPrice
	} else if err != nil {
		return nil, err
	}

	for _, msg := range inMsgs {
		g.executionState.AddInMessage(msg)
		var gasUsed uint32
		var err error
		if msg.Internal {
			gasUsed, err = g.handleInternalInMessage(msg, &gasPrice)
		} else {
			gasUsed, err = g.handleExternalMessage(msg, &gasPrice)
		}
		g.addReceipt(gasUsed, err)
	}

	for _, msg := range outMsgs {
		// TODO: add inMsgHash support (do we even need it?)
		g.executionState.AddOutMessageForTx(common.EmptyHash, msg)
	}

	return g.finalize()
}

func (g *BlockGenerator) handleMessage(msg *types.Message, payer payer, gasPrice *uint256.Int) (uint32, error) {
	gas := msg.GasLimit.Uint64()
	if err := buyGas(payer, msg, gasPrice); err != nil {
		return 0, err
	}

	var leftOverGas uint64
	var err error
	switch msg.Kind {
	case types.DeployMessageKind:
		leftOverGas, err = g.executionState.HandleDeployMessage(g.txOwner.Ctx, msg)
	case types.ExecutionMessageKind:
		leftOverGas, _, err = g.executionState.HandleExecutionMessage(g.txOwner.Ctx, msg)
	case types.RefundMessageKind:
		err = g.executionState.HandleRefundMessage(g.txOwner.Ctx, msg)
	default:
		panic("unreachable")
	}

	refundGas(payer, msg, leftOverGas, gasPrice)
	return uint32(gas - leftOverGas), err
}

func (g *BlockGenerator) validateInternalMessage(message *types.Message) error {
	check.PanicIfNot(message.Internal)

	fromId := message.From.ShardId()
	data, err := g.executionState.Accessor.Access(g.txOwner.RoTx, fromId).GetOutMessage().ByHash(message.Hash())
	if err != nil {
		return err
	}
	if data.Message() == nil {
		return ErrInternalMessageValidationFailed
	}

	switch message.Kind {
	case types.DeployMessageKind:
		return ValidateDeployMessage(message)
	case types.ExecutionMessageKind:
		return nil
	case types.RefundMessageKind:
		return nil
	default:
		panic("unreachable")
	}
}

func (g *BlockGenerator) handleInternalInMessage(msg *types.Message, gasPrice *uint256.Int) (uint32, error) {
	if err := g.validateInternalMessage(msg); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal message")
		return 0, err
	}

	return g.handleMessage(msg, messagePayer{msg, g.executionState}, gasPrice)
}

func (g *BlockGenerator) handleExternalMessage(msg *types.Message, gasPrice *uint256.Int) (uint32, error) {
	if err := ValidateExternalMessage(g.executionState, msg, gasPrice); err != nil {
		g.logger.Trace().Err(err).Msg("Invalid external message")
		return 0, err
	}

	acc, err := g.executionState.GetAccount(msg.To)
	if err != nil {
		return 0, err
	}
	return g.handleMessage(msg, accountPayer{acc, msg}, gasPrice)
}

func (g *BlockGenerator) addReceipt(gasUsed uint32, err error) {
	msgHash := g.executionState.InMessageHash
	msg := g.executionState.GetInMessage()

	if gasUsed == 0 && !msg.Internal {
		check.PanicIfNot(err != nil)

		g.executionState.DropInMessage()
		FailureReceiptCache.Add(msgHash, &types.Receipt{
			Success:         false,
			MsgHash:         msgHash,
			ContractAddress: msg.To,
		})

		g.logger.Debug().
			Err(err).
			Stringer(logging.FieldMessageHash, msgHash).
			Stringer(logging.FieldMessageTo, msg.To).
			Msg("Cached non-authorized fail receipt.")
		return
	}

	g.executionState.AddReceipt(gasUsed, err)

	if err != nil {
		g.logger.Debug().
			Err(err).
			Stringer(logging.FieldMessageHash, msgHash).
			Stringer(logging.FieldMessageTo, msg.To).
			Msg("Added fail receipt.")
	}
}

func (g *BlockGenerator) setLastBlockHashes() error {
	if types.IsMasterShard(g.params.ShardId) {
		for i := 1; i < g.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(g.txOwner.RoTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			g.executionState.SetShardHash(shardId, lastBlockHash)
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(g.txOwner.RoTx, types.MasterShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		g.executionState.SetMasterchainHash(lastBlockHash)
	}
	return nil
}

func (g *BlockGenerator) finalize() (*types.Block, error) {
	if err := g.setLastBlockHashes(); err != nil {
		return nil, err
	}

	blockId := types.BlockNumber(0)
	if !g.executionState.PrevBlock.Empty() {
		b, err := db.ReadBlock(g.txOwner.RoTx, g.params.ShardId, g.executionState.PrevBlock)
		if err != nil {
			return nil, err
		}
		blockId = b.Id + 1
	}

	blockHash, err := g.executionState.Commit(blockId)
	if err != nil {
		return nil, err
	}

	return PostprocessBlock(g.txOwner.RwTx, g.params.ShardId, g.params.GasBasePrice, g.params.GasPriceScale, blockHash)
}
