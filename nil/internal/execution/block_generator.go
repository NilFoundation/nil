package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
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
	PrevBlockId   types.BlockNumber   `json:"prevBlockId"`
	PrevBlockHash common.Hash         `json:"prevBlockHash"`
	CollatorState types.CollatorState `json:"collatorState"`
	MainChainHash common.Hash         `json:"mainChainHash"`
	ShardHashes   []common.Hash       `json:"shardHashes" ssz-max:"4096"`

	InTxns      []*types.Transaction `json:"inTxns" ssz-max:"4096"`
	ForwardTxns []*types.Transaction `json:"forwardTxns" ssz-max:"4096"`

	// In the future, collator should remove transactions from the pool itself after the consensus on the proposal.
	// Currently, we need to remove them after the block was committed, or they may be lost.
	RemoveFromPool []*types.Transaction `json:"removeFromPool" ssz-max:"4096"`
}

func NewEmptyProposal() *Proposal {
	return &Proposal{}
}

func (p *Proposal) IsEmpty() bool {
	return len(p.InTxns) == 0 && len(p.ForwardTxns) == 0
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

type BlockGenerationResult struct {
	Block   *types.Block
	InTxns  []*types.Transaction
	OutTxns []*types.Transaction
}

func NewBlockGenerator(ctx context.Context, params BlockGeneratorParams, txFabric db.DB) (*BlockGenerator, error) {
	rwTx, err := txFabric.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}

	configAccessor, err := config.NewConfigAccessor(ctx, txFabric, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}
	executionState, err := NewExecutionState(rwTx, params.ShardId, StateParams{
		GetBlockFromDb: true,
		Timer:          params.Timer,
		GasPriceScale:  params.GasPriceScale,
		ConfigAccessor: configAccessor,
	})
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

func (g *BlockGenerator) updateGasPrices(prevBlockHash common.Hash, shards []common.Hash) error {
	if g.params.ShardId.IsMainShard() {
		// In main shard we collect gas prices from all shards. Gas price for the main shard is not required.
		gasPrice, err := config.GetParamGasPrice(g.executionState.GetConfigAccessor())
		if err != nil {
			return err
		}
		gasPrice.Shards = make([]types.Uint256, len(shards)+1)
		err = func() error {
			roTx, err := g.txFabric.CreateRoTx(g.ctx)
			if err != nil {
				return err
			}
			defer roTx.Rollback()

			for i := range len(shards) + 1 {
				shardId := types.ShardId(i)
				var shardHash common.Hash
				if shardId.IsMainShard() {
					shardHash = prevBlockHash
				} else {
					shardHash = shards[i-1]
				}

				block, err := db.ReadBlock(roTx, shardId, shardHash)
				if err != nil {
					logger.Err(err).
						Stringer(logging.FieldShardId, shardId).
						Msg("Get gas price from shard: failed to read last block")
					gasPrice.Shards[shardId] = *types.DefaultGasPrice.Uint256
				} else {
					gasPrice.Shards[shardId] = *block.GasPrice.Uint256
				}
			}
			if err = config.SetParamGasPrice(g.executionState.GetConfigAccessor(), gasPrice); err != nil {
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

	res, err := g.finalize(0, nil)
	if err != nil {
		return nil, err
	}
	return res.Block, nil
}

func (g *BlockGenerator) prepareExecutionState(proposal *Proposal, counters *BlockGeneratorCounters, logger zerolog.Logger) error {
	if g.executionState.PrevBlock != proposal.PrevBlockHash {
		// This shouldn't happen currently, because a new block cannot appear between collator and block generator calls.
		esJson, err := g.executionState.MarshalJSON()
		if err != nil {
			logger.Err(err).Msg("Failed to marshal execution state")
			esJson = nil
		}
		//nolint:musttag
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

		return fmt.Errorf("Proposed previous block hash doesn't match the current state. Expected: %s, got: %s",
			g.executionState.PrevBlock, proposal.PrevBlockHash)
	}

	if err := g.updateGasPrices(proposal.PrevBlockHash, proposal.ShardHashes); err != nil {
		return fmt.Errorf("failed to update gas prices: %w", err)
	}

	g.executionState.MainChainHash = proposal.MainChainHash

	var res *ExecutionResult
	for _, txn := range proposal.InTxns {
		if txn.IsDeploy() {
			counters.DeployTransactions++
		}
		if txn.IsExecution() {
			counters.ExecTransactions++
		}

		var txnHash common.Hash
		if assert.Enable {
			txnHash = txn.Hash()
		}

		g.executionState.AddInTransaction(txn)
		if txn.IsInternal() {
			res = g.handleInternalInTransaction(txn)
			counters.InternalTransactions++
		} else {
			res = g.handleExternalTransaction(txn)
			counters.ExternalTransactions++
		}

		if assert.Enable {
			check.PanicIfNotf(txnHash == txn.Hash(), "Transaction hash changed during execution")
		}

		if res.FatalError != nil {
			return res.FatalError
		}
		g.addReceipt(res)
		counters.CoinsUsed = counters.CoinsUsed.Add(res.CoinsUsed())
	}

	for _, txn := range proposal.ForwardTxns {
		// setting all to the same empty hash preserves ordering
		g.executionState.AppendOutTransactionForTx(common.EmptyHash, txn)
	}

	g.executionState.ChildChainBlocks = make(map[types.ShardId]common.Hash, len(proposal.ShardHashes))
	for i, shardHash := range proposal.ShardHashes {
		g.executionState.ChildChainBlocks[types.ShardId(i+1)] = shardHash
	}
	return nil
}

func (g *BlockGenerator) BuildBlock(proposal *Proposal, logger zerolog.Logger) (*types.Block, error) {
	counters := NewBlockGeneratorCounters()

	if err := g.prepareExecutionState(proposal, counters, logger); err != nil {
		return nil, err
	}

	block, _, err := g.executionState.BuildBlock(proposal.PrevBlockId + 1)
	if err != nil {
		return nil, err
	}
	return block, err
}

func (g *BlockGenerator) GenerateBlock(proposal *Proposal, logger zerolog.Logger, sig types.Signature) (*BlockGenerationResult, error) {
	counters := NewBlockGeneratorCounters()

	g.mh.StartProcessingMeasurement(g.ctx, g.executionState.GasPrice, proposal.PrevBlockId+1)
	defer func() { g.mh.EndProcessingMeasurement(g.ctx, counters) }()

	if err := g.prepareExecutionState(proposal, counters, logger); err != nil {
		return nil, err
	}

	if err := db.WriteCollatorState(g.rwTx, g.params.ShardId, proposal.CollatorState); err != nil {
		return nil, fmt.Errorf("failed to write collator state: %w", err)
	}

	return g.finalize(proposal.PrevBlockId+1, sig)
}

func ValidateInternalTransaction(transaction *types.Transaction) error {
	check.PanicIfNot(transaction.IsInternal())

	if transaction.IsDeploy() {
		return ValidateDeployTransaction(transaction)
	}
	return nil
}

func (g *BlockGenerator) handleInternalInTransaction(txn *types.Transaction) *ExecutionResult {
	if err := ValidateInternalTransaction(txn); err != nil {
		g.logger.Warn().Err(err).Msg("Invalid internal transaction")
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorValidation, err))
	}

	return g.executionState.HandleTransaction(g.ctx, txn, NewTransactionPayer(txn, g.executionState))
}

func (g *BlockGenerator) handleExternalTransaction(txn *types.Transaction) *ExecutionResult {
	verifyResult := ValidateExternalTransaction(g.executionState, txn)
	if verifyResult.Failed() {
		g.logger.Error().Err(verifyResult.Error).Msg("External transaction validation failed.")
		return verifyResult
	}

	acc, err := g.executionState.GetAccount(txn.To)
	// Validation cached the account.
	check.PanicIfErr(err)

	res := g.executionState.HandleTransaction(g.ctx, txn, NewAccountPayer(acc, txn))
	res.AddUsed(verifyResult.GasUsed)
	return res
}

func (g *BlockGenerator) addReceipt(execResult *ExecutionResult) {
	check.PanicIfNot(execResult.FatalError == nil)

	txnHash := g.executionState.InTransactionHash
	txn := g.executionState.GetInTransaction()

	if execResult.GasUsed == 0 && txn.IsExternal() {
		// External transactions that don't use gas must not appear here.
		// todo: fail generation here when collator performs full validation.
		check.PanicIfNot(execResult.Failed())

		g.executionState.DropInTransaction()
		AddFailureReceipt(txnHash, txn.To, execResult)

		g.logger.Warn().
			Err(execResult.GetError()).
			Stringer(logging.FieldTransactionHash, txnHash).
			Msg("Encountered unauthenticated failure. Collator must filter out such transactions.")

		return
	}
	g.executionState.AddReceipt(execResult)

	if execResult.Failed() {
		g.logger.Warn().
			Err(execResult.Error).
			Stringer(logging.FieldTransactionHash, txnHash).
			Stringer(logging.FieldTransactionTo, txn.To).
			Msg("Added fail receipt.")
	}
}

func (g *BlockGenerator) finalize(blockId types.BlockNumber, sig types.Signature) (*BlockGenerationResult, error) {
	blockHash, outTxns, err := g.executionState.Commit(blockId, sig)
	if err != nil {
		return nil, err
	}

	block, err := PostprocessBlock(g.rwTx, g.params.ShardId, g.params.GasBasePrice, blockHash)
	if err != nil {
		return nil, err
	}

	ts, err := g.rwTx.CommitWithTs()
	if err != nil {
		return nil, fmt.Errorf("failed to commit block: %w", err)
	}

	// TODO: We should perform block commit and timestamp write atomically.
	tx, err := g.txFabric.CreateRwTx(g.ctx)
	if err != nil {
		return nil, err
	}

	if err := db.WriteBlockTimestamp(tx, g.params.ShardId, blockHash, uint64(ts)); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit block timestamp: %w", err)
	}

	return &BlockGenerationResult{
		Block:   block,
		InTxns:  g.executionState.InTransactions,
		OutTxns: outTxns,
	}, nil
}
