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

func (p *Proposal) IsEmpty() bool {
	return len(p.InMsgs) == 0 && len(p.OutMsgs) == 0
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

	txFabric       db.DB
	rwTx           db.RwTx
	executionState *ExecutionState

	logger zerolog.Logger
}

func NewBlockGenerator(ctx context.Context, params BlockGeneratorParams, txFabric db.DB) (*BlockGenerator, error) {
	rwTx, err := txFabric.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	executionState, err := NewExecutionStateForShard(rwTx, params.ShardId, params.Timer, params.GasPriceScale)
	if err != nil {
		return nil, err
	}
	executionState.TraceVm = params.TraceEVM
	return &BlockGenerator{
		ctx:            ctx,
		params:         params,
		txFabric:       txFabric,
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

func (g *BlockGenerator) GenerateBlock(proposal *Proposal) error {
	if g.executionState.PrevBlock != proposal.PrevBlockHash {
		// This shouldn't happen currently, because a new block cannot appear between collator and block generator calls.
		panic("Proposed previous block hash doesn't match the current state.")
	}

	g.executionState.UpdateGasPrice()

	var err error

	for _, msg := range proposal.InMsgs {
		g.executionState.AddInMessage(msg)
		var gasUsed types.Gas
		if msg.IsInternal() {
			gasUsed, err = g.handleInternalInMessage(msg)
		} else {
			gasUsed, err = g.handleExternalMessage(msg)
		}
		if msgErr := (*types.MessageError)(nil); err != nil && !errors.As(err, &msgErr) {
			return err
		} else {
			g.addReceipt(gasUsed, msgErr)
		}
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

func (g *BlockGenerator) validateInternalMessage(message *types.Message) error {
	check.PanicIfNot(message.IsInternal())

	if message.IsDeploy() {
		return ValidateDeployMessage(message)
	}
	return nil
}

func (g *BlockGenerator) handleInternalInMessage(msg *types.Message) (types.Gas, error) {
	if err := g.validateInternalMessage(msg); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal message")
		return 0, types.NewMessageError(types.MessageStatusValidation, err)
	}

	return g.executionState.handleMessage(g.ctx, msg, messagePayer{msg, msg.FeeCredit.ToGas(g.executionState.GasPrice), g.executionState})
}

func (g *BlockGenerator) handleExternalMessage(msg *types.Message) (types.Gas, error) {
	verifyGas, validationErr, err := ValidateExternalMessage(g.executionState, msg)
	if err != nil {
		g.logger.Error().Err(err).Msg("External message validation failed.")
		return 0, err
	}
	if validationErr != nil {
		g.logger.Error().Err(validationErr).Msg("Invalid external message.")
		return 0, types.NewMessageError(types.MessageStatusValidation, err)
	}

	acc, err := g.executionState.GetAccount(msg.To)
	// Validation cached the account.
	check.PanicIfErr(err)

	usedGas, err := g.executionState.handleMessage(g.ctx, msg, accountPayer{acc, msg})
	return verifyGas.Add(usedGas), err
}

func (g *BlockGenerator) addReceipt(gasUsed types.Gas, err *types.MessageError) {
	msgHash := g.executionState.InMessageHash
	msg := g.executionState.GetInMessage()

	if gasUsed == 0 && msg.IsExternal() {
		// External messages that don't use gas must not appear here.
		// todo: fail generation here when collator performs full validation.
		check.PanicIfNot(err != nil)

		g.executionState.DropInMessage()
		AddFailureReceipt(msgHash, msg.To, err)

		g.logger.Warn().Stringer(logging.FieldMessageHash, msgHash).
			Msg("Encountered unauthenticated failure. Collator must filter out such messages.")

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

	ts, err := g.rwTx.CommitWithTs()
	if err != nil {
		return err
	}

	// TODO: We should perform block commit and timestamp write atomically.
	tx, err := g.txFabric.CreateRwTx(g.ctx)
	if err != nil {
		return err
	}

	if err := db.WriteBlockTimestamp(tx, g.params.ShardId, blockHash, uint64(ts)); err != nil {
		return err
	}

	return tx.Commit()
}
