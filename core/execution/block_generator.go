package execution

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type BlockGeneratorParams struct {
	ShardId       types.ShardId
	NShards       int
	TraceEVM      bool
	Timer         common.Timer
	GasBasePrice  types.Value
	GasPriceScale float64
}

func NewBlockGeneratorParams(shardId types.ShardId, nShards int, gasBasePrice types.Value, gasPriceScale float64) BlockGeneratorParams {
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

func (g *BlockGenerator) GenerateBlock(inMsgs, outMsgs []*types.Message, defaultGasPrice types.Value) (*types.Block, error) {
	gasPrice, err := db.ReadGasPerShard(g.txOwner.RoTx, g.executionState.ShardId)
	if errors.Is(err, db.ErrKeyNotFound) {
		gasPrice = defaultGasPrice
	} else if err != nil {
		return nil, err
	}

	for _, msg := range inMsgs {
		g.executionState.AddInMessage(msg)
		var gasUsed types.Gas
		var err error
		if msg.IsInternal() {
			gasUsed, err = g.handleInternalInMessage(msg, gasPrice)
		} else {
			gasUsed, err = g.handleExternalMessage(msg, gasPrice)
		}
		g.addReceipt(gasUsed, err)
	}

	for _, msg := range outMsgs {
		// TODO: add inMsgHash support (do we even need it?)
		g.executionState.AddOutMessageForTx(common.EmptyHash, msg)
	}

	return g.finalize()
}

func (g *BlockGenerator) handleMessage(msg *types.Message, payer payer, gasPrice types.Value) (types.Gas, error) {
	gas := msg.GasLimit
	if err := buyGas(payer, msg, gasPrice); err != nil {
		return 0, err
	}
	if err := msg.VerifyFlags(); err != nil {
		return 0, err
	}

	var leftOverGas types.Gas
	var err error
	switch {
	case msg.IsDeploy():
		leftOverGas, err = g.executionState.HandleDeployMessage(g.txOwner.Ctx, msg)
		refundGas(payer, msg, leftOverGas, gasPrice)
	case msg.IsRefund():
		err = g.executionState.HandleRefundMessage(g.txOwner.Ctx, msg)
	default:
		leftOverGas, _, err = g.executionState.HandleExecutionMessage(g.txOwner.Ctx, msg)
		refundGas(payer, msg, leftOverGas, gasPrice)
	}

	return gas.Sub(leftOverGas), err
}

func (g *BlockGenerator) validateInternalMessage(message *types.Message) error {
	check.PanicIfNot(message.IsInternal())

	fromId := message.From.ShardId()
	data, err := g.executionState.Accessor.Access(g.txOwner.RoTx, fromId).GetOutMessage().ByHash(message.Hash())
	if err != nil {
		return err
	}
	if data.Message() == nil {
		return ErrInternalMessageValidationFailed
	}

	if message.IsDeploy() {
		return ValidateDeployMessage(message)
	}
	return nil
}

func (g *BlockGenerator) handleInternalInMessage(msg *types.Message, gasPrice types.Value) (types.Gas, error) {
	if err := g.validateInternalMessage(msg); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal message")
		return 0, err
	}

	return g.handleMessage(msg, messagePayer{msg, g.executionState}, gasPrice)
}

func (g *BlockGenerator) handleExternalMessage(msg *types.Message, gasPrice types.Value) (types.Gas, error) {
	if err := ValidateExternalMessage(g.executionState, msg, gasPrice); err != nil {
		g.logger.Error().Err(err).Msg("Invalid external message.")
		return 0, err
	}

	acc, err := g.executionState.GetAccount(msg.To)
	if err != nil {
		return 0, err
	}
	return g.handleMessage(msg, accountPayer{acc, msg}, gasPrice)
}

func (g *BlockGenerator) addReceipt(gasUsed types.Gas, err error) {
	msgHash := g.executionState.InMessageHash
	msg := g.executionState.GetInMessage()

	if gasUsed == 0 && msg.IsExternal() {
		check.PanicIfNot(err != nil)

		// todo: this is a temporary solution, we shouldn't store errors for unpaid failures
		g.executionState.DropInMessage()
		FailureReceiptCache.Add(msgHash, ReceiptWithError{
			Receipt: &types.Receipt{
				Success:         false,
				MsgHash:         msgHash,
				ContractAddress: msg.To,
			},
			Error: err,
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
