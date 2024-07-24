package execution

import (
	"context"
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

type Proposal struct {
	PrevBlockId   types.BlockNumber
	PrevBlockHash common.Hash
	CollatorState types.CollatorState
	MainChainHash common.Hash
	ShardHashes   map[types.ShardId]common.Hash

	InMsgs  []*types.Message
	OutMsgs []*types.Message

	// In the future, collator should remove messages from the pool itself after the consensus on the proposal.
	// Currently, we need to remove them after the block was committed, or they may be lost.
	RemoveFromPool []*types.Message
}

func NewEmptyProposal() *Proposal {
	return &Proposal{
		ShardHashes: make(map[types.ShardId]common.Hash),
	}
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
	ctx    context.Context
	params BlockGeneratorParams

	rwTx           db.RwTx
	executionState *ExecutionState

	logger zerolog.Logger
}

func NewBlockGenerator(ctx context.Context, params BlockGeneratorParams, txFabric db.DB) (*BlockGenerator, error) {
	rwTx, err := txFabric.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	executionState, err := NewExecutionStateForShard(rwTx, params.ShardId, params.Timer)
	if err != nil {
		return nil, err
	}
	executionState.TraceVm = params.TraceEVM
	return &BlockGenerator{
		ctx:            ctx,
		params:         params,
		rwTx:           rwTx,
		executionState: executionState,
		logger: logging.NewLogger("block-gen").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
	}, nil
}

func (g *BlockGenerator) Rollback() {
	g.rwTx.Rollback()
}

func (g *BlockGenerator) GenerateZeroState(zeroState string) error {
	g.logger.Info().Msg("Generating zero-state...")

	if err := g.executionState.GenerateZeroState(zeroState); err != nil {
		return err
	}

	err := g.finalize(0)
	return err
}

func (g *BlockGenerator) GenerateBlock(proposal *Proposal, defaultGasPrice types.Value) error {
	if g.executionState.PrevBlock != proposal.PrevBlockHash {
		// This shouldn't happen currently, because a new block cannot appear between collator and block generator calls.
		panic("Proposed previous block hash doesn't match the current state.")
	}

	var err error
	gasPrice, err := db.ReadGasPerShard(g.rwTx, g.executionState.ShardId)
	if errors.Is(err, db.ErrKeyNotFound) {
		gasPrice = defaultGasPrice
	} else if err != nil {
		return err
	}

	for _, msg := range proposal.InMsgs {
		g.executionState.AddInMessage(msg)
		var gasUsed types.Gas
		if msg.IsInternal() {
			gasUsed, err = g.handleInternalInMessage(msg, gasPrice)
		} else {
			gasUsed, err = g.handleExternalMessage(msg, gasPrice)
		}
		g.addReceipt(gasUsed, err)
	}

	for _, msg := range proposal.OutMsgs {
		// TODO: add inMsgHash support (do we even need it?)
		g.executionState.AppendOutMessageForTx(common.EmptyHash, msg)
	}

	g.executionState.MainChainHash = proposal.MainChainHash
	g.executionState.ChildChainBlocks = proposal.ShardHashes

	if err := db.WriteCollatorState(g.rwTx, g.params.ShardId, proposal.CollatorState); err != nil {
		return err
	}

	return g.finalize(proposal.PrevBlockId + 1)
}

func (g *BlockGenerator) handleMessage(msg *types.Message, payer payer, gasPrice types.Value) (types.Gas, error) {
	gas := msg.GasLimit
	if err := buyGas(payer, msg, gasPrice); err != nil {
		return 0, types.NewMessageError(types.MessageStatusBuyGas, err)
	}
	if err := msg.VerifyFlags(); err != nil {
		return 0, types.NewMessageError(types.MessageStatusValidation, err)
	}

	var leftOverGas types.Gas
	var err error
	switch {
	case msg.IsDeploy():
		leftOverGas, err = g.executionState.HandleDeployMessage(g.ctx, msg)
		refundGas(payer, msg, leftOverGas, gasPrice)
	case msg.IsRefund():
		err = g.executionState.HandleRefundMessage(g.ctx, msg)
	default:
		leftOverGas, _, err = g.executionState.HandleExecutionMessage(g.ctx, msg)
		refundGas(payer, msg, leftOverGas, gasPrice)
	}

	return gas.Sub(leftOverGas), err
}

func (g *BlockGenerator) validateInternalMessage(message *types.Message) error {
	check.PanicIfNot(message.IsInternal())

	if message.IsDeploy() {
		return ValidateDeployMessage(message)
	}
	return nil
}

func (g *BlockGenerator) handleInternalInMessage(msg *types.Message, gasPrice types.Value) (types.Gas, error) {
	if err := g.validateInternalMessage(msg); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal message")
		return 0, types.NewMessageError(types.MessageStatusValidation, err)
	}

	return g.handleMessage(msg, messagePayer{msg, g.executionState}, gasPrice)
}

func (g *BlockGenerator) handleExternalMessage(msg *types.Message, gasPrice types.Value) (types.Gas, error) {
	if err := ValidateExternalMessage(g.executionState, msg, gasPrice); err != nil {
		g.logger.Error().Err(err).Msg("Invalid external message.")
		return 0, types.NewMessageError(types.MessageStatusValidation, err)
	}

	acc, err := g.executionState.GetAccount(msg.To)
	if err != nil {
		return 0, types.NewMessageError(types.MessageStatusNoAccount, err)
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

func (g *BlockGenerator) finalize(blockId types.BlockNumber) error {
	blockHash, err := g.executionState.Commit(blockId)
	if err != nil {
		return err
	}

	if _, err := PostprocessBlock(g.rwTx, g.params.ShardId, g.params.GasBasePrice, g.params.GasPriceScale, blockHash); err != nil {
		return err
	}

	if err := g.rwTx.Commit(); err != nil {
		return err
	}

	return nil
}
