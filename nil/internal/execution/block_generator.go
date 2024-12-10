package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

type BlockGeneratorParams struct {
	ShardId       types.ShardId
	NShards       uint32
	TraceEVM      bool
	Timer         common.Timer
	GasBasePrice  types.Value
	GasPriceScale float64
}

type Proposal struct {
	PrevBlockId   types.BlockNumber             `json:"prevBlockId"`
	PrevBlockHash common.Hash                   `json:"prevBlockHash"`
	CollatorState types.CollatorState           `json:"-"`
	MainChainHash common.Hash                   `json:"mainChainHash"`
	ShardHashes   map[types.ShardId]common.Hash `json:"shardHashes"`

	InMsgs      []*types.Message `json:"inMsgs"`
	ForwardMsgs []*types.Message `json:"forwardMsgs"`

	// In the future, collator should remove messages from the pool itself after the consensus on the proposal.
	// Currently, we need to remove them after the block was committed, or they may be lost.
	RemoveFromPool []*types.Message `json:"-"`
}

func NewEmptyProposal() *Proposal {
	return &Proposal{
		ShardHashes: make(map[types.ShardId]common.Hash),
	}
}

func (p *Proposal) IsEmpty() bool {
	return len(p.InMsgs) == 0 && len(p.ForwardMsgs) == 0
}

func NewBlockGeneratorParams(shardId types.ShardId, nShards uint32, gasBasePrice types.Value, gasPriceScale float64) BlockGeneratorParams {
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
	mh     *MetricsHandler
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

	const mhName = "github.com/NilFoundation/nil/nil/internal/execution"
	mh, err := NewMetricsHandler(mhName, params.ShardId)
	if err != nil {
		return nil, err
	}

	return &BlockGenerator{
		ctx:            ctx,
		params:         params,
		txFabric:       txFabric,
		rwTx:           rwTx,
		executionState: executionState,
		logger: logging.NewLogger("block-gen").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
		mh: mh,
	}, nil
}

func (g *BlockGenerator) Rollback() {
	g.rwTx.Rollback()
}

func (g *BlockGenerator) updateGasPrices(numShards int) error {
	if g.params.ShardId.IsMainShard() {
		// In main shard we collect gas prices from all shards. Gas price for the main shard is not required.
		gasPrice, err := g.executionState.GetConfigAccessor().GetParamGasPrice()
		if err != nil {
			return err
		}
		gasPrice.Shards = make([]types.Uint256, numShards)
		err = func() error {
			roTx, err := g.txFabric.CreateRoTx(g.ctx)
			if err != nil {
				return err
			}
			defer roTx.Rollback()

			for i := range numShards {
				shardId := types.ShardId(i)
				_ = shardId
				block, _, err := db.ReadLastBlock(roTx, shardId)
				if err != nil {
					return err
				}
				gasPrice.Shards[i] = *block.GasPrice.Uint256
			}
			if err = g.executionState.GetConfigAccessor().SetParamGasPrice(gasPrice); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return fmt.Errorf("failed to read gas prices from shards: %w", err)
		}
	} else {
		// In regular shards, we calculate new gas price for the current block.
		g.executionState.UpdateGasPrice()
	}
	return nil
}

func (g *BlockGenerator) GenerateZeroState(zeroStateYaml string, config *ZeroStateConfig) (*types.Block, error) {
	g.logger.Info().Msg("Generating zero-state...")

	if config != nil {
		if err := g.executionState.GenerateZeroState(config); err != nil {
			return nil, err
		}
	} else if err := g.executionState.GenerateZeroStateYaml(zeroStateYaml); err != nil {
		return nil, err
	}

	b, _, err := g.finalize(0)
	return b, err
}

func (g *BlockGenerator) GenerateBlock(proposal *Proposal, logger zerolog.Logger) (*types.Block, []*types.Message, error) {
	if g.executionState.PrevBlock != proposal.PrevBlockHash {
		// This shouldn't happen currently, because a new block cannot appear between collator and block generator calls.
		esJson, err := g.executionState.MarshalJSON()
		if err != nil {
			logger.Err(err).Msg("Failed to marshal execution state")
			esJson = nil
		}
		proposalJson, err := json.Marshal(proposal)
		if err != nil {
			logger.Err(err).Msg("Failed to marshal block proposal")
			proposalJson = nil
		}

		logger.Debug().
			Stringer("expected", g.executionState.PrevBlock).
			Stringer("got", proposal.PrevBlockHash).
			RawJSON("executionState", esJson).
			RawJSON("proposal", proposalJson).
			Msg("Proposed previous block hash doesn't match the current state")

		panic(
			fmt.Sprintf("Proposed previous block hash doesn't match the current state. Expected: %s, got: %s",
				g.executionState.PrevBlock, proposal.PrevBlockHash),
		)
	}

	if err := g.updateGasPrices(len(proposal.ShardHashes) + 1); err != nil {
		return nil, nil, fmt.Errorf("failed to update gas prices: %w", err)
	}

	g.executionState.MainChainHash = proposal.MainChainHash

	var res *ExecutionResult

	counters := NewBlockGeneratorCounters()
	g.mh.StartProcessingMeasurement(g.ctx, g.executionState.GasPrice, proposal.PrevBlockId+1)
	defer func() { g.mh.EndProcessingMeasurement(g.ctx, counters) }()

	for _, msg := range proposal.InMsgs {
		if msg.IsDeploy() {
			counters.DeployMessages++
		}
		if msg.IsExecution() {
			counters.ExecMessages++
		}
		g.executionState.AddInMessage(msg)
		if msg.IsInternal() {
			res = g.handleInternalInMessage(msg)
			counters.InternalMessages++
		} else {
			res = g.handleExternalMessage(msg)
			counters.ExternalMessages++
		}
		if res.FatalError != nil {
			return nil, nil, res.FatalError
		}
		g.addReceipt(res)
		counters.CoinsUsed = counters.CoinsUsed.Add(res.CoinsUsed())
	}

	for _, msg := range proposal.ForwardMsgs {
		// setting all to the same empty hash preserves ordering
		g.executionState.AppendOutMessageForTx(common.EmptyHash, msg)
	}

	g.executionState.ChildChainBlocks = proposal.ShardHashes

	if err := db.WriteCollatorState(g.rwTx, g.params.ShardId, proposal.CollatorState); err != nil {
		return nil, nil, err
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

func (g *BlockGenerator) handleInternalInMessage(msg *types.Message) *ExecutionResult {
	if err := g.validateInternalMessage(msg); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal message")
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorValidation, err))
	}

	return g.executionState.HandleMessage(g.ctx, msg, NewMessagePayer(msg, g.executionState))
}

func (g *BlockGenerator) handleExternalMessage(msg *types.Message) *ExecutionResult {
	verifyResult := ValidateExternalMessage(g.executionState, msg)
	if verifyResult.Failed() {
		g.logger.Error().Err(verifyResult.Error).Msg("External message validation failed.")
		return verifyResult
	}

	acc, err := g.executionState.GetAccount(msg.To)
	// Validation cached the account.
	check.PanicIfErr(err)

	res := g.executionState.HandleMessage(g.ctx, msg, NewAccountPayer(acc, msg))
	res.AddUsed(verifyResult.GasUsed)
	return res
}

func (g *BlockGenerator) addReceipt(execResult *ExecutionResult) {
	check.PanicIfNot(execResult.FatalError == nil)

	msgHash := g.executionState.InMessageHash
	msg := g.executionState.GetInMessage()

	if execResult.GasUsed == 0 && msg.IsExternal() {
		// External messages that don't use gas must not appear here.
		// todo: fail generation here when collator performs full validation.
		check.PanicIfNot(execResult.Failed())

		g.executionState.DropInMessage()
		AddFailureReceipt(msgHash, msg.To, execResult)

		g.logger.Warn().Stringer(logging.FieldMessageHash, msgHash).
			Msg("Encountered unauthenticated failure. Collator must filter out such messages.")

		return
	}
	g.executionState.AddReceipt(execResult)

	if execResult.Failed() {
		g.logger.Debug().
			Err(execResult.Error).
			Stringer(logging.FieldMessageHash, msgHash).
			Stringer(logging.FieldMessageTo, msg.To).
			Msg("Added fail receipt.")
	}
}

func (g *BlockGenerator) finalize(blockId types.BlockNumber) (*types.Block, []*types.Message, error) {
	blockHash, outMsgs, err := g.executionState.Commit(blockId)
	if err != nil {
		return nil, nil, err
	}

	block, err := PostprocessBlock(g.rwTx, g.params.ShardId, g.params.GasBasePrice, blockHash)
	if err != nil {
		return nil, nil, err
	}

	ts, err := g.rwTx.CommitWithTs()
	if err != nil {
		return nil, nil, err
	}

	// TODO: We should perform block commit and timestamp write atomically.
	tx, err := g.txFabric.CreateRwTx(g.ctx)
	if err != nil {
		return nil, nil, err
	}

	if err := db.WriteBlockTimestamp(tx, g.params.ShardId, blockHash, uint64(ts)); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return block, outMsgs, nil
}
