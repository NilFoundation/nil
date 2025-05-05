package fetching

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/blob"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/constraints"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/jonboulle/clockwork"
)

type AggregatorMetrics interface {
	srv.WorkerMetrics
	RecordBatchCreated(ctx context.Context, batch *types.BlockBatch)
}

type BatchConstraintChecker interface {
	Constraints() constraints.BatchConstraints
	CheckConstraints(ctx context.Context, batch *types.BlockBatch) (*constraints.CheckResult, error)
}

type AggregatorTaskStorage interface {
	AddTaskEntries(ctx context.Context, tasks ...*types.TaskEntry) error
}

type AggregatorBlockStorage interface {
	GetLatestFetched(ctx context.Context) (types.BlockRefs, error)
	TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error)
	TryGetLatestBatch(ctx context.Context) (*types.BlockBatch, error)
	PutBlockBatch(ctx context.Context, batch *types.BlockBatch) error
}

type AggregatorConfig struct {
	RpcPollingInterval time.Duration `yaml:"pollingDelay,omitempty"`
	MaxBlobsInTx       uint          `yaml:"-"`
}

func NewAggregatorConfig(rpcPollingInterval time.Duration) AggregatorConfig {
	return AggregatorConfig{
		RpcPollingInterval: rpcPollingInterval,
		MaxBlobsInTx:       6,
	}
}

func NewDefaultAggregatorConfig() AggregatorConfig {
	return NewAggregatorConfig(time.Second)
}

type aggregator struct {
	concurrent.Suspendable

	fetcher         *Fetcher
	batchChecker    BatchConstraintChecker
	blockStorage    AggregatorBlockStorage
	taskStorage     AggregatorTaskStorage
	subgraphFetcher *subgraphFetcher
	batchEncoder    encode.BatchEncoder
	blobBuilder     blob.Builder
	rollupContract  rollupcontract.Wrapper
	resetter        *reset.StateResetLauncher
	clock           clockwork.Clock
	metrics         AggregatorMetrics
	config          AggregatorConfig
	logger          logging.Logger
}

func NewAggregator(
	fetcher *Fetcher,
	batchChecker BatchConstraintChecker,
	blockStorage AggregatorBlockStorage,
	taskStorage AggregatorTaskStorage,
	resetter *reset.StateResetLauncher,
	rollupContractWrapper rollupcontract.Wrapper,
	clock clockwork.Clock,
	logger logging.Logger,
	metrics AggregatorMetrics,
	config AggregatorConfig,
) *aggregator {
	agg := &aggregator{
		fetcher:         fetcher,
		batchChecker:    batchChecker,
		blockStorage:    blockStorage,
		taskStorage:     taskStorage,
		subgraphFetcher: newSubgraphFetcher(fetcher, logger),
		batchEncoder:    v1.NewEncoder(logger),
		blobBuilder:     blob.NewBuilder(),
		rollupContract:  rollupContractWrapper,
		resetter:        resetter,
		clock:           clock,
		metrics:         metrics,
		config:          config,
	}

	iteration := srv.NewWorkerIteration(logger, metrics, agg.Name(), agg.processBlocksAndHandleErr)
	agg.Suspendable = concurrent.NewSuspendable(iteration.Run, config.RpcPollingInterval)
	agg.logger = srv.WorkerLogger(logger, agg)
	return agg
}

func (agg *aggregator) Name() string {
	return "aggregator"
}

// processBlocksAndHandleErr fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *aggregator) processBlocksAndHandleErr(ctx context.Context) error {
	err := agg.processBlockRange(ctx)
	return agg.handleProcessingErr(ctx, err)
}

func (agg *aggregator) handleProcessingErr(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, types.ErrBlockMismatch):
		agg.logger.Warn().Err(err).Msg("Block mismatch detected, resetting state")
		if err := agg.resetter.LaunchResetToL1WithSuspension(ctx, agg); err != nil {
			return fmt.Errorf("error resetting state: %w", err)
		}
		return nil

	case errors.Is(err, types.ErrStateRootNotInitialized):
		agg.logger.Warn().Err(err).Msg("Local state root not initialized, skipping")
		return nil

	case errors.Is(err, storage.ErrCapacityLimitReached):
		agg.logger.Info().Err(err).Msg("Storage capacity limit reached, skipping")
		return nil

	default:
		return err
	}
}

// processBlockRange handles the processing of new/pending blocks across all shards.
// It fetches blocks, creates batches, and updates the storage.
func (agg *aggregator) processBlockRange(ctx context.Context) error {
	batch, err := agg.tryPrepareBatch(ctx)
	if err != nil {
		return fmt.Errorf("error preparing batch: %w", err)
	}
	if batch == nil {
		agg.logger.Debug().Msg("Unable to prepare batch, skipping")
		return nil
	}

	agg.logger.Debug().Stringer(logging.FieldBatchId, batch.Id).Msg("Extending batch with new blocks")

	fetchingRange, err := agg.getFetchingRange(ctx)
	if err != nil {
		return fmt.Errorf("error getting fetching range: %w", err)
	}
	if fetchingRange == nil {
		return nil
	}

	result := agg.extendBatchWithRange(ctx, batch, *fetchingRange)
	if result.err != nil {
		return fmt.Errorf("error extending batch, batchId=%s: %w", batch.Id, result.err)
	}

	if result.extended.IsEmpty() {
		agg.logger.Debug().Stringer(logging.FieldBatchId, result.extended.Id).Msg("Batch is empty, skipping")
		return nil
	}

	if result.shouldBeSealed {
		if err := agg.sealBatch(ctx, result.extended); err != nil {
			return fmt.Errorf("error sealing batch, batchId=%s: %w", result.extended.Id, err)
		}
	} else {
		// batch can be further extended, just putting in to the storage
		if err := agg.blockStorage.PutBlockBatch(ctx, result.extended); err != nil {
			return fmt.Errorf("error storing batch, batchId=%s: %w", result.extended.Id, err)
		}
	}

	return nil
}

func (agg *aggregator) tryPrepareBatch(ctx context.Context) (*types.BlockBatch, error) {
	latestBatch, err := agg.blockStorage.TryGetLatestBatch(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading latest batch from the storage: %w", err)
	}

	if latestBatch == nil {
		agg.logger.Info().Msg("No batches found in storage, starting from scratch")
		return agg.createAndPutNewBatch(ctx, nil)
	}

	if latestBatch.IsSealed {
		agg.logger.Info().Msgf("Latest batch with id=%s is sealed, creating the next one", latestBatch.Id)
		return agg.createAndPutNewBatch(ctx, &latestBatch.Id)
	}

	agg.logger.Debug().Msgf("Latest batch with id=%s is not sealed", latestBatch.Id)

	checkResult, err := agg.batchChecker.CheckConstraints(ctx, latestBatch)
	if err != nil {
		return nil, fmt.Errorf("error checking batch constraints, batchId=%s: %w", latestBatch.Id, err)
	}

	switch checkResult.Type {
	case constraints.CheckResultTypeShouldBeDiscarded:
		agg.logger.Warn().
			Stringer(logging.FieldBatchId, latestBatch.Id).
			Msgf("Discarding latest batch due to constraint(s) violation: %s", checkResult.Details)

		if err := agg.resetter.LaunchPartialResetWithSuspension(ctx, agg, latestBatch.Id); err != nil {
			return nil, fmt.Errorf("error resetting progress for batch %s: %w", latestBatch.Id, err)
		}

		return nil, nil

	case constraints.CheckResultTypeShouldBeSealed:
		agg.logger.Info().
			Stringer(logging.FieldBatchId, latestBatch.Id).
			Msgf("Sealing batch: %s", checkResult.Details)

		if err := agg.sealBatch(ctx, latestBatch); err != nil {
			return nil, fmt.Errorf("error sealing batch: %w", err)
		}

		return nil, nil

	case constraints.CheckResultTypeCanBeExtended:
		return latestBatch, nil

	default:
		return nil, fmt.Errorf("unexpected batch check result type: %s, batchId=%s", checkResult.Type, latestBatch.Id)
	}
}

func (agg *aggregator) createAndPutNewBatch(ctx context.Context, parentId *types.BatchId) (*types.BlockBatch, error) {
	now := agg.clock.Now()
	nextBatch := types.NewBlockBatch(parentId, now)

	err := agg.blockStorage.PutBlockBatch(ctx, nextBatch)
	switch {
	case errors.Is(err, storage.ErrCapacityLimitReached):
		return nil, fmt.Errorf("%w, cannot create new batch", err)

	case errors.Is(err, context.Canceled):
		return nil, err

	case err != nil:
		return nil, fmt.Errorf("error storing new batch, batchId=%s: %w", nextBatch.Id, err)

	default:
		return nextBatch, nil
	}
}

func (agg *aggregator) getFetchingRange(ctx context.Context) (*types.BlocksRange, error) {
	startingBlockRef, err := agg.getStartingBlockRef(ctx)
	if err != nil {
		return nil, err
	}

	latestBlockRef, err := agg.fetcher.GetLatestBlockRef(ctx, coreTypes.MainShardId)
	if err != nil {
		return nil, err
	}

	rangeLimit := agg.batchChecker.Constraints().MaxBlocksCount
	fetchingRange, err := types.GetBlocksFetchingRange(*startingBlockRef, *latestBlockRef, rangeLimit)
	if err != nil {
		return nil, err
	}

	if fetchingRange == nil {
		agg.logger.Debug().
			Stringer(logging.FieldShardId, coreTypes.MainShardId).
			Stringer(logging.FieldBlockNumber, latestBlockRef.Number).
			Msg("No new blocks to fetch")
	}

	return fetchingRange, nil
}

// getStartingBlockRef retrieves the starting point for the next fetching iteration,
// prioritizing the latest fetched main shard block if available.
// If the `latestFetched` value is not defined, method uses `latestProvedStateRoot`.
// If neither of the two values is defined, method returns an error.
func (agg *aggregator) getStartingBlockRef(ctx context.Context) (*types.BlockRef, error) {
	latestFetched, err := agg.blockStorage.GetLatestFetched(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading latest fetched block for the main shard: %w", err)
	}
	if mainRef := latestFetched.TryGetMain(); mainRef != nil {
		// checking if `latestFetched` still exists on the L2 side
		if err := agg.checkIfExistsOnL2(ctx, mainRef); err != nil {
			return nil, err
		}

		return mainRef, nil
	}

	agg.logger.Debug().Msg("No blocks fetched yet, latest proved state root value will be used")

	latestProvedRoot, err := agg.blockStorage.TryGetProvedStateRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("error reading latest proved state root: %w", err)
	}
	if latestProvedRoot == nil {
		return nil, types.ErrStateRootNotInitialized
	}

	ref, err := agg.fetcher.TryGetBlockRef(ctx, coreTypes.MainShardId, *latestProvedRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get proved block ref: %w", err)
	}
	if ref == nil {
		return nil, fmt.Errorf("proved block ref not found, stateRoot=%s", latestProvedRoot)
	}
	return ref, nil
}

func (agg *aggregator) checkIfExistsOnL2(ctx context.Context, mainRef *types.BlockRef) error {
	blockRef, err := agg.fetcher.TryGetBlockRef(ctx, mainRef.ShardId, mainRef.Hash)
	if err != nil {
		return fmt.Errorf("fetched block check error: %w", err)
	}
	if blockRef == nil {
		return fmt.Errorf(
			"%w: fetched block not found on L2 side, blockRef=%s", types.ErrBlockMismatch, mainRef,
		)
	}
	return nil
}

type batchExtensionResult struct {
	extended       *types.BlockBatch
	shouldBeSealed bool
	err            error
}

func extensionErr(reason error) batchExtensionResult {
	return batchExtensionResult{err: reason}
}

func (agg *aggregator) extendBatchWithRange(
	ctx context.Context,
	batch *types.BlockBatch,
	blocksRange types.BlocksRange,
) batchExtensionResult {
	latestFetched, err := agg.blockStorage.GetLatestFetched(ctx)
	if err != nil {
		return extensionErr(err)
	}

	const shardId = coreTypes.MainShardId
	expendedBatch := batch
	for mainShardBlock, err := range agg.fetcher.FetchBlocksSeq(ctx, shardId, blocksRange) {
		if err != nil {
			return extensionErr(fmt.Errorf("error fetching block from shard %d: %w", shardId, err))
		}

		result := agg.extendBatch(ctx, expendedBatch, mainShardBlock, latestFetched)
		if result.err != nil || result.shouldBeSealed {
			return result
		}

		expendedBatch = result.extended
		for shard, ref := range expendedBatch.LatestRefs() {
			latestFetched[shard] = ref
		}
	}

	return batchExtensionResult{extended: expendedBatch, shouldBeSealed: false}
}

func (agg *aggregator) extendBatch(
	ctx context.Context,
	batch *types.BlockBatch,
	mainShardBlock *types.Block,
	latestFetched types.BlockRefs,
) batchExtensionResult {
	subgraph, err := agg.subgraphFetcher.FetchSubgraph(ctx, mainShardBlock, latestFetched)
	if err != nil {
		return extensionErr(err)
	}

	now := agg.clock.Now()
	extendedBatch, err := batch.WithAddedBlocks(subgraph, now)
	if err != nil {
		return extensionErr(fmt.Errorf("failed to add new blocks: %w", err))
	}

	checkResult, err := agg.batchChecker.CheckConstraints(ctx, extendedBatch)
	if err != nil {
		return extensionErr(fmt.Errorf("error checking batch constraints: %w", err))
	}

	switch checkResult.Type {
	case constraints.CheckResultTypeCanBeExtended:
		return batchExtensionResult{extended: extendedBatch, shouldBeSealed: false}

	case constraints.CheckResultTypeShouldBeDiscarded:
		return batchExtensionResult{extended: batch, shouldBeSealed: true}

	case constraints.CheckResultTypeShouldBeSealed:
		return batchExtensionResult{extended: extendedBatch, shouldBeSealed: true}

	default:
		return extensionErr(fmt.Errorf("unexpected batch check result type: %s", checkResult.Type))
	}
}

// sealBatch handles the batch, preparing data proofs, committing to the rollup contract, and creating proof tasks.
func (agg *aggregator) sealBatch(ctx context.Context, batch *types.BlockBatch) error {
	sidecar, dataProofs, err := agg.prepareForBatchCommit(ctx, batch)
	if err != nil {
		return err
	}

	sealedBatch, err := batch.Seal(dataProofs, agg.clock.Now())
	if err != nil {
		return err
	}

	if err := agg.blockStorage.PutBlockBatch(ctx, sealedBatch); err != nil {
		return fmt.Errorf("error storing batch, batchId=%s: %w", batch.Id, err)
	}

	if err := agg.rollupContract.CommitBatch(ctx, sidecar, sealedBatch.Id.String()); err != nil {
		return agg.handleCommitBatchError(ctx, sealedBatch, err)
	}

	if err := agg.createProofTasks(ctx, sealedBatch); err != nil {
		return fmt.Errorf("error creating proof tasks, batchId=%s: %w", batch.Id, err)
	}

	agg.metrics.RecordBatchCreated(ctx, sealedBatch)
	return nil
}

func (agg *aggregator) handleCommitBatchError(ctx context.Context, batch *types.BlockBatch, err error) error {
	switch {
	case errors.Is(err, rollupcontract.ErrBatchAlreadyCommitted) ||
		errors.Is(err, rollupcontract.ErrBatchAlreadyFinalized):
		// for some reason, we attempted to prove a batch that has already been proved,
		// sync the latest proved root with the L1 contract.
		agg.logger.Warn().Stringer(logging.FieldBatchId, batch.Id).
			Err(err).Msg("batch is already committed, resetting state with L1")
		if err := agg.resetter.LaunchResetToL1WithSuspension(ctx, agg); err != nil {
			return fmt.Errorf("error resetting state from L1, latestMainHash=%s: %w",
				batch.LatestMainBlock().Hash, err)
		}
	case errors.Is(err, rollupcontract.ErrInvalidBatchIndex) ||
		errors.Is(err, rollupcontract.ErrInvalidVersionedHash):
		// NOTE: this shouldn't happen in prod setting
		agg.logger.Error().Stringer(logging.FieldBatchId, batch.Id).
			Err(err).Msg("data was corrupted or initially created in a wrong way")
		if err := agg.resetter.LaunchResetToL1WithSuspension(ctx, agg); err != nil {
			return fmt.Errorf("error resetting state from L1, latestMainHash=%s: %w",
				batch.LatestMainBlock().Hash, err)
		}
	default:
		return err
	}
	return nil
}

// createProofTask generates proof task for block batch
func (agg *aggregator) createProofTasks(ctx context.Context, batch *types.BlockBatch) error {
	currentTime := agg.clock.Now()
	proofTask, err := batch.CreateProofTask(currentTime)
	if err != nil {
		return fmt.Errorf("error creating proof tasks: %w", err)
	}

	if err := agg.taskStorage.AddTaskEntries(ctx, proofTask); err != nil {
		return fmt.Errorf("error adding task entry: %w", err)
	}

	agg.logger.Debug().Stringer(logging.FieldBatchId, batch.Id).Msgf("Created proof task, batchId=%s", batch.Id)

	return nil
}

func (agg *aggregator) prepareForBatchCommit(
	ctx context.Context, batch *types.BlockBatch,
) (*ethtypes.BlobTxSidecar, types.DataProofs, error) {
	var binTransactions bytes.Buffer
	if err := agg.batchEncoder.Encode(types.NewPrunedBatch(batch), &binTransactions); err != nil {
		return nil, nil, err
	}
	agg.logger.Debug().Int("compressed_batch_len", binTransactions.Len()).Msg("encoded transaction")

	blobs, err := agg.blobBuilder.MakeBlobs(&binTransactions, agg.config.MaxBlobsInTx)
	if err != nil {
		return nil, nil, err
	}

	return agg.rollupContract.PrepareBlobs(ctx, blobs)
}
