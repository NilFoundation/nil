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
	ShardId      types.ShardId
	NShards      uint32
	TraceEVM     bool
	MainKeysPath string
}

func NewBlockGeneratorParams(shardId types.ShardId, nShards uint32) BlockGeneratorParams {
	return BlockGeneratorParams{
		ShardId: shardId,
		NShards: nShards,
	}
}

type BlockGenerator struct {
	ctx    context.Context
	params BlockGeneratorParams

	txFabric       db.DB
	rwTx           db.RwTx
	executionState *ExecutionState

	logger   zerolog.Logger
	mh       *MetricsHandler
	counters *BlockGeneratorCounters
}

type BlockGenerationResult struct {
	Block        *types.Block
	BlockHash    common.Hash
	OutTxns      []*types.Transaction
	InTxns       []*types.Transaction
	ConfigParams map[string][]byte
}

func NewBlockGenerator(ctx context.Context, params BlockGeneratorParams, txFabric db.DB, blockHash, mainShardHash *common.Hash) (*BlockGenerator, error) {
	rwTx, err := txFabric.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}

	configAccessor, err := config.NewConfigAccessor(ctx, txFabric, mainShardHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	getBlockFromDb := false
	if blockHash == nil {
		getBlockFromDb = true
		blockHash = &common.EmptyHash
	}

	executionState, err := NewExecutionState(rwTx, params.ShardId, StateParams{
		BlockHash:      *blockHash,
		GetBlockFromDb: getBlockFromDb,
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
		mh:       mh,
		counters: NewBlockGeneratorCounters(),
	}, nil
}

func (g *BlockGenerator) Rollback() {
	g.rwTx.Rollback()
}

func (p *BlockGenerator) CollectGasPrices(prevBlockHash common.Hash, shardHashes []common.Hash) []types.Uint256 {
	if !p.params.ShardId.IsMainShard() {
		return nil
	}

	// In main shard we collect gas prices from all shards. Gas price for the main shard is not required.
	shards := make([]types.Uint256, len(shardHashes)+1)
	for i := range shards {
		shardId := types.ShardId(i)
		var shardHash common.Hash
		if shardId.IsMainShard() {
			shardHash = prevBlockHash
		} else {
			shardHash = shardHashes[i-1]
		}

		block, err := db.ReadBlock(p.rwTx, shardId, shardHash)
		if err != nil {
			p.logger.Err(err).
				Stringer(logging.FieldShardId, shardId).
				Msg("Get gas price from shard: failed to read last block")
			shards[shardId] = *types.DefaultGasPrice.Uint256
		} else {
			shards[shardId] = *block.BaseFee.Uint256
		}
	}
	return shards
}

func (g *BlockGenerator) updateGasPrices(gasPrices []types.Uint256) error {
	if !g.params.ShardId.IsMainShard() {
		return nil
	}

	gasPriceParam := &config.ParamGasPrice{
		Shards: gasPrices,
	}
	if err := config.SetParamGasPrice(g.executionState.GetConfigAccessor(), gasPriceParam); err != nil {
		return fmt.Errorf("failed to set gas prices: %w", err)
	}

	// In main shard we don't need to update base fee.
	g.executionState.BaseFee = types.DefaultGasPrice
	return nil
}

func (g *BlockGenerator) GenerateZeroState(zeroStateYaml string, config *ZeroStateConfig) (*types.Block, error) {
	g.logger.Info().Msg("Generating zero-state...")
	g.executionState.BaseFee = types.DefaultGasPrice

	if !g.params.ShardId.IsMainShard() {
		mainBlockHash, err := db.ReadBlockHashByNumber(g.rwTx, types.MainShardId, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to read main block hash: %w", err)
		}
		g.executionState.MainChainHash = mainBlockHash
	}

	if err := g.executionState.GenerateMergedZeroState(config, zeroStateYaml); err != nil {
		return nil, err
	}

	res, err := g.finalize(0, nil)
	if err != nil {
		return nil, err
	}
	g.logger.Info().Msg("Zero-state generated")
	return res.Block, nil
}

func (g *BlockGenerator) prepareExecutionState(proposal *Proposal, gasPrices []types.Uint256) error {
	if g.executionState.PrevBlock != proposal.PrevBlockHash {
		esJson, err := g.executionState.MarshalJSON()
		if err != nil {
			g.logger.Err(err).Msg("Failed to marshal execution state")
			esJson = nil
		}

		proposalJson, err := json.Marshal(proposal)
		if err != nil {
			g.logger.Err(err).Msg("Failed to marshal block proposal")
			proposalJson = nil
		}

		g.logger.Debug().
			Stringer("expected", g.executionState.PrevBlock).
			Stringer("got", proposal.PrevBlockHash).
			RawJSON("executionState", esJson).
			RawJSON("proposal", proposalJson).
			Msg("Proposed previous block hash doesn't match the current state")

		err = fmt.Errorf("proposed previous block hash doesn't match the current state. Expected: %s, got: %s",
			g.executionState.PrevBlock, proposal.PrevBlockHash)
		if assert.Enable {
			panic(err)
		}
		return err
	}

	if err := g.updateGasPrices(gasPrices); err != nil {
		return fmt.Errorf("failed to update gas prices: %w", err)
	}

	g.executionState.MainChainHash = proposal.MainChainHash

	for _, txn := range proposal.InternalTxns {
		if err := g.handleTxn(txn); err != nil {
			return err
		}
	}

	for _, txn := range proposal.ExternalTxns {
		if err := g.handleTxn(txn); err != nil {
			return err
		}
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

func (g *BlockGenerator) handleTxn(txn *types.Transaction) error {
	if txn.IsDeploy() {
		g.counters.DeployTransactions++
	}
	if txn.IsExecution() {
		g.counters.ExecTransactions++
	}

	var txnHash common.Hash
	if assert.Enable {
		txnHash = txn.Hash()
	}

	var res *ExecutionResult
	g.executionState.AddInTransaction(txn)
	if txn.IsInternal() {
		res = g.handleInternalInTransaction(txn)
		g.counters.InternalTransactions++
	} else {
		res = g.handleExternalTransaction(txn)
		g.counters.ExternalTransactions++
	}

	if assert.Enable {
		check.PanicIfNotf(txnHash == txn.Hash(), "Transaction hash changed during execution")
	}

	if res.FatalError != nil {
		return res.FatalError
	}
	g.addReceipt(res)
	g.counters.CoinsUsed = g.counters.CoinsUsed.Add(res.CoinsUsed())

	return nil
}

func (g *BlockGenerator) BuildBlock(proposal *Proposal, gasPrices []types.Uint256) (*BlockGenerationResult, error) {
	if err := g.prepareExecutionState(proposal, gasPrices); err != nil {
		return nil, err
	}
	return g.executionState.BuildBlock(proposal.PrevBlockId + 1)
}

func (g *BlockGenerator) GenerateBlock(proposal *Proposal, sig *types.BlsAggregateSignature) (*BlockGenerationResult, error) {
	g.mh.StartProcessingMeasurement(g.ctx, g.executionState.GasPrice, proposal.PrevBlockId+1)
	defer func() { g.mh.EndProcessingMeasurement(g.ctx, g.counters) }()

	gasPrices := g.CollectGasPrices(proposal.PrevBlockHash, proposal.ShardHashes)
	if err := g.prepareExecutionState(proposal, gasPrices); err != nil {
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

func (g *BlockGenerator) finalize(blockId types.BlockNumber, sig *types.BlsAggregateSignature) (*BlockGenerationResult, error) {
	blockRes, err := g.executionState.BuildBlock(blockId)
	if err != nil {
		return nil, err
	}

	return blockRes, g.Finalize(blockRes, sig)
}

func (g *BlockGenerator) Finalize(blockRes *BlockGenerationResult, sig *types.BlsAggregateSignature) error {
	if err := g.executionState.CommitBlock(blockRes.Block, sig); err != nil {
		return err
	}

	if err := PostprocessBlock(g.rwTx, g.params.ShardId, blockRes); err != nil {
		return err
	}

	ts, err := g.rwTx.CommitWithTs()
	if err != nil {
		return fmt.Errorf("failed to commit block: %w", err)
	}

	// TODO: We should perform block commit and timestamp write atomically.
	tx, err := g.txFabric.CreateRwTx(g.ctx)
	if err != nil {
		return err
	}

	if err := db.WriteBlockTimestamp(tx, g.params.ShardId, blockRes.BlockHash, uint64(ts)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit block timestamp: %w", err)
	}

	return nil
}
