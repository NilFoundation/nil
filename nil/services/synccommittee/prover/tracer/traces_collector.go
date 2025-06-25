package tracer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
)

type RemoteTracesCollector interface {
	GetBlockTraces(ctx context.Context, blockId BlockId) (*ExecutionTraces, error)
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
	client          client.Client
	logger          logging.Logger
	mptTracer       *mpttracer.MPTTracer
	rwTx            db.RwTx
	blockAccessor   *execution.BlockAccessor
	lastTracedBlock *types.BlockNumber
}

var _ RemoteTracesCollector = (*remoteTracesCollectorImpl)(nil)

// NewRemoteTracesCollector creates a new instance of RemoteTracesCollector
func NewRemoteTracesCollector(
	ctx context.Context,
	client client.Client,
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
		client:        client,
		logger:        logger,
		rwTx:          rwTx,
		blockAccessor: execution.NewBlockAccessor(32),
	}, nil
}

// initMptTracer initializes the MPT tracer with the given block number and contract trie root
func (tc *remoteTracesCollectorImpl) initMptTracer(
	shardId types.ShardId,
	startBlockNum types.BlockNumber,
	contractTrieRoot common.Hash,
) error {
	tc.mptTracer = mpttracer.New(tc.client, startBlockNum, tc.rwTx, shardId, tc.logger)
	return tc.mptTracer.SetRootHash(contractTrieRoot)
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

	decodedBlock, err := dbgBlock.DecodeBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode block: %w", err)
	}

	return dbgBlock, decodedBlock, nil
}

func decodeTxns(
	tx db.RwTx, shardId types.ShardId, txns []*types.Transaction, counts []*types.TxCount,
) error {
	txTrie := execution.NewDbTransactionTrie(tx, shardId)

	txnKeys := make([]types.TransactionIndex, 0, len(txns))
	txnValues := make([]*types.Transaction, 0, len(txns))
	for i, txn := range txns {
		txnKeys = append(txnKeys, types.TransactionIndex(i))
		txnValues = append(txnValues, txn)
	}

	if err := txTrie.UpdateBatch(txnKeys, txnValues); err != nil {
		return fmt.Errorf("failed to update tx trie: %w", err)
	}

	rootHash, err := txTrie.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit tx trie: %w", err)
	}

	txCountKeys := make([]types.ShardId, 0, len(counts))
	txCountValues := make([]*types.TransactionIndex, 0, len(counts))
	for _, c := range counts {
		if c.Count > 0 {
			txCountKeys = append(txCountKeys, types.ShardId(c.ShardId))
			txCountValues = append(txCountValues, &c.Count)
		}
	}

	txCountTrie := execution.NewDbTxCountTrie(tx, shardId)
	if err := txCountTrie.SetRootHash(rootHash); err != nil {
		return fmt.Errorf("failed to set root hash for tx count trie: %w", err)
	}

	if err := txCountTrie.UpdateBatch(txCountKeys, txCountValues); err != nil {
		return fmt.Errorf("failed to update tx count trie: %w", err)
	}

	if _, err := txCountTrie.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx count trie: %w", err)
	}

	return nil
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
	// TODO: to collect single MPT trace for multiple sequential block, MPTTracer instance should be kept between calls.
	// Currently, MPT traces will contain only the last traced block. Since there is no MPT circuit yet,
	// it's not a big deal.
	if err := tc.initMptTracer(shardId, prevBlock.Id, prevBlock.SmartContractsRoot); err != nil {
		return nil, fmt.Errorf("failed to initialize MPT tracer: %w", err)
	}

	configAccessor := config.NewConfigAccessorFromMap(configMap)
	if err := decodeTxns(tc.rwTx, shardId, prevBlock.InTransactions, prevBlock.InTxCounts); err != nil {
		return nil, err
	}
	if err := decodeTxns(tc.rwTx, shardId, prevBlock.OutTransactions, prevBlock.OutTxCounts); err != nil {
		return nil, err
	}

	es, err := execution.NewExecutionState(
		tc.rwTx,
		shardId,
		execution.StateParams{
			Block:                 prevBlock.Block,
			ConfigAccessor:        configAccessor,
			BlockAccessor:         tc.blockAccessor,
			ContractMptRepository: tc.mptTracer,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution state: %w", err)
	}

	esTracer := NewEVMTracer(es)

	// Set tracers in execution state
	es.EvmTracingHooks = esTracer.getTracingHooks()

	// Create block generator params
	blockGeneratorParams := execution.NewBlockGeneratorParams(shardId, uint32(len(gasPrices)))
	blockGeneratorParams.EvmTracingHooks = es.EvmTracingHooks
	blockGeneratorParams.BlockAccessor = tc.blockAccessor

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

	mptTraces, err := tc.mptTracer.GetMPTTraces()
	if err != nil {
		return nil, fmt.Errorf("error getting mpt traces: %w", err)
	}
	esTracer.Traces.MPTTraces = &mptTraces

	zethCache, err := tc.mptTracer.GetZethCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting zeth cache: %w", err)
	}
	esTracer.Traces.ZethCache = zethCache

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
	rootHash, err := configTrie.Commit()
	if err != nil {
		return err
	}
	return configTrie.SetRootHash(rootHash)
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

// CollectTraces collects traces for blocks within the range specified in config. Traces are not written to a file,
// thus, `MarshalMode` and `BaseFileName` fields of the config are not used and could be omitted.
// Blocks in `BlockIDs` config field must be sequential, otherwise, `ErrBlocksNotSequential` will be raised.
func CollectTraces(ctx context.Context, client client.Client, cfg *TraceConfig) (*ExecutionTraces, error) {
	version, err := client.ClientVersion(ctx)
	if err != nil {
		return nil, err
	}

	getBlockCaches := make([]mpttracer.GetBlockCache, 0, len(cfg.BlockIDs)+1)

	remoteTracesCollector, err := NewRemoteTracesCollector(ctx, client, logging.NewLogger("tracer"))
	if err != nil {
		return nil, err
	}
	aggregatedTraces := NewExecutionTraces()
	firstBlockId := cfg.BlockIDs[0]
	prevBlockNum := *firstBlockId.Id.BlockNumber - 1
	prevBlock, err := client.GetBlock(ctx, firstBlockId.ShardId, prevBlockNum, true)
	if err != nil {
		return nil, err
	}
	getBlockCache := mpttracer.GetBlockCache{
		Args: mpttracer.BlockArgs{
			BlockNo: prevBlockNum.Uint64(),
			ShardID: uint64(firstBlockId.ShardId),
		},
		Block: *prevBlock,
	}
	getBlockCaches = append(getBlockCaches, getBlockCache)
	for _, blockID := range cfg.BlockIDs {
		rpcBlock, err := client.GetBlock(ctx, blockID.ShardId, blockID.Id, true)
		if err != nil {
			return nil, err
		}
		getBlockCache := mpttracer.GetBlockCache{
			Args: mpttracer.BlockArgs{
				BlockNo: blockID.Id.BlockNumber.Uint64(),
				ShardID: uint64(blockID.ShardId),
			},
			Block: *rpcBlock,
		}
		getBlockCaches = append(getBlockCaches, getBlockCache)

		traces, err := remoteTracesCollector.GetBlockTraces(ctx, blockID)
		if err != nil {
			return nil, err
		}
		aggregatedTraces.Append(traces)
	}

	aggregatedTraces.ZethCache.ClientVersion = version
	aggregatedTraces.ZethCache.FullBlocks = getBlockCaches
	// aggregatedTraces.ZethCache.Receipts = // TODO

	return aggregatedTraces, nil
}

func CollectTracesToFile(ctx context.Context, client client.Client, cfg *TraceConfig) error {
	traces, err := CollectTraces(ctx, client, cfg)
	if err != nil {
		return err
	}

	return SerializeToFile(traces, cfg.MarshalMode, cfg.BaseFileName)
}
