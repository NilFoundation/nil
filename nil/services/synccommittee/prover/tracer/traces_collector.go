package tracer

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
)

type RemoteTracesCollector interface {
	GetBlockTraces(ctx context.Context, blockId BlockId) (*ExecutionTraces, error)
	GetMPTTraces() (mpttracer.MPTTraces, error)
}

type BlockId struct {
	ShardId types.ShardId
	Id      transport.BlockReference
}

// TraceConfig holds configuration for trace collection
type TraceConfig struct {
	BlockIDs     []BlockId
	BaseFileName string
	MarshalMode  MarshalMode
}

// remoteTracesCollectorImpl implements RemoteTracesCollector interface
type remoteTracesCollectorImpl struct {
	client          api.RpcClient
	logger          logging.Logger
	mptTracer       *mpttracer.MPTTracer
	rwTx            db.RwTx
	lastTracedBlock *types.BlockNumber
}

var _ RemoteTracesCollector = (*remoteTracesCollectorImpl)(nil)

// NewRemoteTracesCollector creates a new instance of RemoteTracesCollector
func NewRemoteTracesCollector(
	ctx context.Context,
	client api.RpcClient,
	logger logging.Logger,
) (RemoteTracesCollector, error) {
	localDb, err := db.NewBadgerDbInMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory DB: %w", err)
	}

	rwTx, err := localDb.CreateRwTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB transaction: %w", err)
	}

	return &remoteTracesCollectorImpl{
		client: client,
		logger: logger,
		rwTx:   rwTx,
	}, nil
}

// initMptTracer initializes the MPT tracer with the given block number and contract trie root
func (tc *remoteTracesCollectorImpl) initMptTracer(
	shardId types.ShardId,
	startBlockNum types.BlockNumber,
	contractTrieRoot common.Hash,
) {
	tc.mptTracer = mpttracer.New(tc.client, startBlockNum, tc.rwTx, shardId)
	tc.mptTracer.SetRootHash(contractTrieRoot)
}

// GetBlockTraces retrieves the traces for a single block.
// It requires that blocks are processed sequentially.
func (tc *remoteTracesCollectorImpl) GetBlockTraces(
	ctx context.Context,
	blockId BlockId,
) (*ExecutionTraces, error) {
	tc.logger.Debug().
		Stringer("blockRef", blockId.Id).
		Stringer(logging.FieldShardId, blockId.ShardId).
		Msg("collecting traces for block")

	// Get current block
	_, currentDbgBlock, err := tc.fetchAndDecodeBlock(ctx, blockId.ShardId, blockId.Id)
	if err != nil {
		return nil, err
	}

	// Handle genesis block
	if currentDbgBlock.Id == 0 {
		// TODO: prove genesis block generation?
		return nil, ErrCantProofGenesisBlock
	}

	// Ensure blocks are sequential
	if tc.lastTracedBlock != nil && currentDbgBlock.Id != *tc.lastTracedBlock+1 {
		return nil, fmt.Errorf("%w: previous block number: %d, current block number: %d",
			ErrBlocksNotSequential, *tc.lastTracedBlock, currentDbgBlock.Id)
	}
	tc.lastTracedBlock = &currentDbgBlock.Id

	// Get previous block
	_, prevDbgBlock, err := tc.fetchAndDecodeBlock(
		ctx, blockId.ShardId, transport.BlockNumber(currentDbgBlock.Id-1).AsBlockReference(),
	)
	if err != nil {
		return nil, err
	}

	// Get configuration and gas prices for block
	configMap, gasPrices, err := tc.getConfigForBlock(ctx, blockId.ShardId, currentDbgBlock.Block)
	if err != nil {
		return nil, err
	}

	// Write previous block to DB for `ExecutionState` to read, execution fails otherwise
	if err := db.WriteBlock(
		tc.rwTx, blockId.ShardId, prevDbgBlock.Hash(blockId.ShardId), prevDbgBlock.Block,
	); err != nil {
		return nil, fmt.Errorf("failed to write previous block to DB: %w", err)
	}

	// Initialize execution state, collect traces
	traces, err := tc.executeBlockAndCollectTraces(
		ctx, blockId.ShardId, currentDbgBlock, prevDbgBlock, configMap, gasPrices,
	)
	if err != nil {
		return nil, err
	}

	return traces, nil
}

// fetchAndDecodeBlock fetches a block from debug API and decodes it
func (tc *remoteTracesCollectorImpl) fetchAndDecodeBlock(
	ctx context.Context,
	shardId types.ShardId,
	blockRef transport.BlockReference,
) (*jsonrpc.DebugRPCBlock, *types.BlockWithExtractedData, error) {
	dbgBlock, err := tc.client.GetDebugBlock(ctx, shardId, blockRef, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get debug block: %w", err)
	}
	if dbgBlock == nil {
		return nil, nil, ErrClientReturnedNilBlock
	}

	decodedBlock, err := dbgBlock.DecodeSSZ()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode block: %w", err)
	}

	return dbgBlock, decodedBlock, nil
}

func decodeTxCounts(counts []*types.TxCountSSZ) execution.TxCounts {
	txCounts := make(execution.TxCounts, len(counts))
	for _, count := range counts {
		txCounts[types.ShardId(count.ShardId)] = count.Count
	}
	return txCounts
}

// executeBlockAndCollectTraces executes the block and collects traces
func (tc *remoteTracesCollectorImpl) executeBlockAndCollectTraces(
	ctx context.Context,
	shardId types.ShardId,
	currentBlock *types.BlockWithExtractedData,
	prevBlock *types.BlockWithExtractedData,
	configMap map[string][]byte,
	gasPrices []types.Uint256,
) (*ExecutionTraces, error) {
	configAccessor := config.NewConfigAccessorFromMap(configMap)

	es, err := execution.NewExecutionState(
		tc.rwTx,
		shardId,
		execution.StateParams{
			Block:          prevBlock.Block,
			ConfigAccessor: configAccessor,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution state: %w", err)
	}
	es.InTxCounts = decodeTxCounts(prevBlock.InTxCounts)
	es.OutTxCounts = decodeTxCounts(prevBlock.OutTxCounts)

	esTracer := NewEVMTracer(es)

	// TODO: to collect single MPT trace for multiple sequential block, MPTTracer instance should be kept between calls.
	// Currently, MPT traces will contain only the last traced block. Since there is no MPT circuit yet,
	// it's not a big deal.
	tc.initMptTracer(shardId, prevBlock.Id, prevBlock.SmartContractsRoot)

	// Set tracers in execution state
	es.ContractTree = tc.mptTracer
	es.EvmTracingHooks = esTracer.getTracingHooks()

	// Create block generator params
	blockGeneratorParams := execution.NewBlockGeneratorParams(shardId, uint32(len(gasPrices)))
	blockGeneratorParams.EvmTracingHooks = es.EvmTracingHooks

	// Create block generator
	blockGenerator, err := execution.NewBlockGeneratorWithEs(
		ctx,
		blockGeneratorParams,
		nil, // txFabric is unused in our case
		tc.rwTx,
		es,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create block generator: %w", err)
	}

	// Create proposal from block data
	proposal := tc.createProposalFromBlocks(shardId, prevBlock, currentBlock)

	// Build block
	tc.logger.Debug().Msg("building block")
	generatedBlock, err := blockGenerator.BuildBlock(&proposal, gasPrices)
	if err != nil {
		if esTracer.TracingError != nil {
			return nil, fmt.Errorf("block generator failed with: %w, tracing error: %w", err, esTracer.TracingError)
		}
		return nil, fmt.Errorf("block generator failed with: %w", err)
	}

	// Check for tracing errors
	if esTracer.TracingError != nil {
		return nil, esTracer.TracingError
	}

	// Validate generated block hash matches expected
	expectedHash := currentBlock.Hash(shardId)
	if generatedBlock.BlockHash != expectedHash {
		return nil, fmt.Errorf("%w: expected hash: %s, generated hash: %s",
			ErrTracedBlockHashMismatch, expectedHash, generatedBlock.BlockHash)
	}

	return esTracer.Traces, nil
}

// createProposalFromBlocks creates an execution proposal from block data
func (tc *remoteTracesCollectorImpl) createProposalFromBlocks(
	shardId types.ShardId,
	prevBlock *types.BlockWithExtractedData,
	currentBlock *types.BlockWithExtractedData,
) execution.Proposal {
	proposal := execution.Proposal{
		PrevBlockId:   prevBlock.Id,
		PrevBlockHash: prevBlock.Hash(shardId),
		CollatorState: types.CollatorState{},
		MainShardHash: currentBlock.MainShardHash,
		ShardHashes:   currentBlock.ChildBlocks,
	}

	proposal.InternalTxns, proposal.ExternalTxns = execution.SplitInTransactions(currentBlock.InTransactions)
	proposal.ForwardTxns, _ = execution.SplitOutTransactions(currentBlock.OutTransactions, shardId)

	return proposal
}

// getConfigForBlock retrieves configuration and gas prices for the given block
func (tc *remoteTracesCollectorImpl) getConfigForBlock(
	ctx context.Context,
	shardId types.ShardId,
	block *types.Block,
) (map[string][]byte, []types.Uint256, error) {
	// Get block with configuration
	blockWithConfigHash := block.GetMainShardHash(shardId)
	blockWithConfigRaw, blockWithConfig, err := tc.fetchAndDecodeBlock(
		ctx, types.MainShardId, transport.HashBlockReference(blockWithConfigHash),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block with config: %w", err)
	}

	// Get raw config data
	configData, err := blockWithConfigRaw.Config.ToMap()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert config to map: %w", err)
	}

	// Populate config trie
	if err := tc.populateConfigTrie(configData); err != nil {
		return nil, nil, err
	}

	// Get gas prices
	gasPrices, err := tc.collectGasPrices(ctx, shardId, blockWithConfig)
	if err != nil {
		return nil, nil, err
	}

	return configData, gasPrices, nil
}

// populateConfigTrie populates the config trie in the database
func (tc *remoteTracesCollectorImpl) populateConfigTrie(configMap map[string][]byte) error {
	configTrie := mpt.NewDbMPT(tc.rwTx, types.MainShardId, db.ConfigTrieTable)
	for k, v := range configMap {
		if err := configTrie.Set([]byte(k), v); err != nil {
			return fmt.Errorf("failed to set config trie key %s: %w", k, err)
		}
	}
	return nil
}

// collectGasPrices collects gas prices for all shards
func (tc *remoteTracesCollectorImpl) collectGasPrices(
	ctx context.Context,
	shardId types.ShardId,
	blockWithConfig *types.BlockWithExtractedData,
) ([]types.Uint256, error) {
	gasPrices := []types.Uint256{}

	// Skip if not main shard
	if !shardId.IsMainShard() {
		return gasPrices, nil
	}

	// Add main shard gas price
	gasPrices = append(gasPrices, *blockWithConfig.BaseFee.Uint256)

	// Get previous block for child shard gas prices (except for genesis)
	blockWithGasPricesId := blockWithConfig.Id
	if blockWithConfig.Id != 0 {
		blockWithGasPricesId--
	}

	_, gasPricesBlock, err := tc.fetchAndDecodeBlock(
		ctx, types.MainShardId, transport.BlockNumber(blockWithGasPricesId).AsBlockReference(),
	)
	if err != nil {
		return nil, err
	}

	// Collect gas prices from child shards
	for i, blockHash := range gasPricesBlock.ChildBlocks {
		childBlock, err := tc.client.GetBlock(ctx, types.ShardId(i), blockHash, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get child block for shard %d: %w", i, err)
		}
		gasPrices = append(gasPrices, *childBlock.BaseFee.Uint256)
	}

	return gasPrices, nil
}

// GetMPTTraces returns the collected MPT traces
func (tc *remoteTracesCollectorImpl) GetMPTTraces() (mpttracer.MPTTraces, error) {
	if tc.mptTracer == nil {
		return mpttracer.MPTTraces{}, errors.New("MPT tracer not initialized")
	}
	return tc.mptTracer.GetMPTTraces()
}

// CollectTraces collects traces for blocks within the range specified in config. Traces are not written to a file,
// thus, `MarshalMode` and `BaseFileName` fields of the config are not used and could be omitted.
// Blocks in `BlockIDs` config field must be sequential, otherwise, `ErrBlocksNotSequential` will be raised.
func CollectTraces(ctx context.Context, rpcClient api.RpcClient, cfg *TraceConfig) (*ExecutionTraces, error) {
	remoteTracesCollector, err := NewRemoteTracesCollector(ctx, rpcClient, logging.NewLogger("tracer"))
	if err != nil {
		return nil, err
	}
	aggregatedTraces := NewExecutionTraces()
	for _, blockID := range cfg.BlockIDs {
		traces, err := remoteTracesCollector.GetBlockTraces(ctx, blockID)
		if err != nil {
			return nil, err
		}
		aggregatedTraces.Append(traces)
	}

	// FIXME: MPT trace aggregates changes from multiple sequential blocks,
	// and can't be constructed from multiple shards.
	mptTraces, err := remoteTracesCollector.GetMPTTraces()
	if err != nil {
		return nil, err
	}
	aggregatedTraces.SetMptTraces(&mptTraces)

	return aggregatedTraces, nil
}

func CollectTracesToFile(ctx context.Context, rpcClient api.RpcClient, cfg *TraceConfig) error {
	traces, err := CollectTraces(ctx, rpcClient, cfg)
	if err != nil {
		return err
	}

	return SerializeToFile(traces, cfg.MarshalMode, cfg.BaseFileName)
}
