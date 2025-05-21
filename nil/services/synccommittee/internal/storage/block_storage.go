package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
)

type BlockStorageMetrics interface {
	RecordBatchProved(ctx context.Context)
}

type BlockStorageConfig struct {
	// StoredBatchesLimit defines the maximum number of stored batches.
	// If the capacity limit is reached, method BlockStorage.SetBlockBatch returns ErrCapacityLimitReached error.
	StoredBatchesLimit uint32
}

func NewBlockStorageConfig(storedBatchesLimit uint32) BlockStorageConfig {
	return BlockStorageConfig{
		StoredBatchesLimit: storedBatchesLimit,
	}
}

func DefaultBlockStorageConfig() BlockStorageConfig {
	return NewBlockStorageConfig(100)
}

type BlockStorage struct {
	commonStorage
	config  BlockStorageConfig
	clock   clockwork.Clock
	metrics BlockStorageMetrics

	ops struct {
		batchOp
		batchCountOp
		batchLatestOp
		blockOp
		blockLatestFetchedOp
		stateRootOp
	}
}

func NewBlockStorage(
	database db.DB,
	config BlockStorageConfig,
	clock clockwork.Clock,
	metrics BlockStorageMetrics,
	logger logging.Logger,
) *BlockStorage {
	return &BlockStorage{
		commonStorage: makeCommonStorage(
			database,
			logger,
			common.DoNotRetryIf(
				scTypes.ErrBatchMismatch, scTypes.ErrBlockMismatch,
				scTypes.ErrBlockNotFound, scTypes.ErrBatchNotFound,
				scTypes.ErrBatchNotProved, scTypes.ErrLocalStateRootNotInitialized,
			),
		),
		config:  config,
		clock:   clock,
		metrics: metrics,
	}
}

func (bs *BlockStorage) TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return bs.ops.getProvedStateRoot(tx)
}

func (bs *BlockStorage) SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error {
	if stateRoot == common.EmptyHash {
		return errors.New("state root cannot be empty")
	}

	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := bs.ops.putProvedStateRoot(tx, stateRoot); err != nil {
		return err
	}

	return bs.commit(tx)
}

// TryGetLatestBatch retrieves the latest created batch
// or returns nil if:
// a) No batches have been created yet, or
// b) A full storage reset (starting from the first batch) has been triggered.
func (bs *BlockStorage) TryGetLatestBatch(ctx context.Context) (*scTypes.BlockBatch, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	batchId, err := bs.ops.getLatestBatchId(tx)
	if err != nil {
		return nil, err
	}
	if batchId == nil {
		return nil, nil
	}

	entry, err := bs.ops.getBatchEntry(tx, *batchId)
	if err != nil {
		return nil, err
	}

	return bs.reconstructBatch(tx, entry)
}

func (bs *BlockStorage) reconstructBatch(tx db.RoTx, entry *batchEntry) (*scTypes.BlockBatch, error) {
	segments, err := bs.ops.getBlocksAsSegments(tx, entry.BlockIds)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate chain segments, batchId=%s: %w", entry.Id, err)
	}

	batch := scTypes.ReconstructExistingBlockBatch(
		entry.Id,
		entry.ParentId,
		segments,
		entry.DataProofs,
		entry.IsSealed,
		entry.CreatedAt,
		entry.UpdatedAt,
	)
	return batch, nil
}

func (bs *BlockStorage) BatchExists(ctx context.Context, batchId scTypes.BatchId) (bool, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	_, err = bs.ops.getBatchEntry(tx, batchId)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, scTypes.ErrBatchNotFound):
		return false, nil
	default:
		return false, err
	}
}

func (bs *BlockStorage) GetBatch(ctx context.Context, batchId scTypes.BatchId) (*scTypes.BlockBatch, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entry, err := bs.ops.getBatchEntry(tx, batchId)
	if err != nil {
		return nil, err
	}
	return bs.reconstructBatch(tx, entry)
}

func (bs *BlockStorage) GetLatestFetched(ctx context.Context) (scTypes.BlockRefs, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return bs.ops.getLatestFetched(tx)
}

func (bs *BlockStorage) TryGetBlock(ctx context.Context, id scTypes.BlockId) (*jsonrpc.RPCBlock, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entry, err := bs.ops.getBlock(tx, id, false)
	if err != nil || entry == nil {
		return nil, err
	}
	return &entry.Block, nil
}

func (bs *BlockStorage) HasFreeSpace(ctx context.Context) (bool, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return bs.ops.hasFreeSpace(tx, bs.config)
}

// PutBlockBatch creates a new batch in the storage or updates an existing one.
func (bs *BlockStorage) PutBlockBatch(ctx context.Context, batch *scTypes.BlockBatch) error {
	if batch == nil {
		return errors.New("batch cannot be nil")
	}

	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.putBlockBatch(ctx, batch)
	})
}

func (bs *BlockStorage) putBlockBatch(ctx context.Context, batch *scTypes.BlockBatch) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	batchExists, err := bs.ops.batchExists(tx, batch.Id)
	if err != nil {
		return err
	}

	if !batchExists {
		if err := bs.ops.addStoredCount(tx, 1, bs.config); err != nil {
			return err
		}
		if err := bs.ops.updateLatestBatchId(tx, batch); err != nil {
			return err
		}
	}

	currentLatestRefs, err := bs.ops.getLatestFetched(tx)
	if err != nil {
		return err
	}

	if err := bs.validateBatchSequencing(tx, currentLatestRefs, batch); err != nil {
		return err
	}

	if err := bs.putBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	if err := bs.updateLatestFetched(tx, currentLatestRefs, batch); err != nil {
		return err
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) updateLatestFetched(
	tx db.RwTx,
	currentLatestRefs scTypes.BlockRefs,
	batch *scTypes.BlockBatch,
) error {
	for shard, batchEarliestRef := range batch.LatestRefs() {
		currentLatestRef := currentLatestRefs.TryGet(shard)
		if currentLatestRef != nil && currentLatestRef.Number >= batchEarliestRef.Number {
			continue
		}

		if err := bs.ops.putLatestFetchedRef(tx, batchEarliestRef.ShardId, &batchEarliestRef); err != nil {
			return err
		}
	}
	return nil
}

func (bs *BlockStorage) validateBatchSequencing(
	tx db.RoTx,
	currentLatestRefs scTypes.BlockRefs,
	batch *scTypes.BlockBatch,
) error {
	for shard, block := range batch.EarliestBlocks() {
		blockId := scTypes.IdFromBlock(block)
		alreadyExists, err := bs.ops.blockExists(tx, blockId)
		if err != nil {
			return err
		}
		if alreadyExists {
			// block has already been saved, no need to validate sequencing
			continue
		}

		currentRef := currentLatestRefs.TryGet(shard)
		if currentRef == nil {
			// no blocks fetched in shard, no need to validate
			continue
		}

		if err := currentRef.ValidateNext(block); err != nil {
			return fmt.Errorf(
				"%w: %w: latestFetched=%s, batchEarliest=%s",
				scTypes.ErrBatchMismatch, err, currentRef, scTypes.BlockToRef(block),
			)
		}
	}

	return nil
}

func (bs *BlockStorage) SetBatchAsProved(ctx context.Context, batchId scTypes.BatchId) error {
	wasSet, err := bs.setBatchAsProvedImpl(ctx, batchId)
	if err != nil {
		return err
	}
	if wasSet {
		bs.metrics.RecordBatchProved(ctx)
	}
	return nil
}

func (bs *BlockStorage) setBatchAsProvedImpl(ctx context.Context, batchId scTypes.BatchId) (wasSet bool, err error) {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	entry, err := bs.ops.getBatchEntry(tx, batchId)
	if err != nil {
		return false, err
	}

	if entry.IsProved {
		bs.logger.Debug().Stringer(logging.FieldBatchId, batchId).Msg("batch is already marked as proved")
		return false, nil
	}

	entry.IsProved = true
	if err := bs.ops.putBatchEntry(tx, entry); err != nil {
		return false, err
	}

	if err := bs.commit(tx); err != nil {
		return false, err
	}

	return true, nil
}

func (bs *BlockStorage) TryGetNextProposalData(ctx context.Context) (*scTypes.ProposalData, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentProvedStateRoot, err := bs.ops.getProvedStateRoot(tx)
	if err != nil {
		return nil, err
	}
	if currentProvedStateRoot == nil {
		return nil, scTypes.ErrLocalStateRootNotInitialized
	}

	var proposalCandidate *batchEntry
	for entry, err := range bs.ops.getStoredBatchesSeq(tx) {
		if err != nil {
			return nil, err
		}
		if entry.IsValidProposalCandidate(*currentProvedStateRoot) {
			proposalCandidate = entry
			break
		}
	}

	if proposalCandidate == nil {
		bs.logger.Debug().Stringer(logging.FieldStateRoot, currentProvedStateRoot).Msg("no proved batch found")
		return nil, nil
	}

	return bs.createProposalDataTx(proposalCandidate, *currentProvedStateRoot)
}

func (bs *BlockStorage) createProposalDataTx(
	proposalCandidate *batchEntry,
	currentProvedStateRoot common.Hash,
) (*scTypes.ProposalData, error) {
	latestMainRef := proposalCandidate.LatestRefs.TryGetMain()
	if latestMainRef == nil {
		return nil, fmt.Errorf("batch with id=%s has no latest main block", proposalCandidate.Id)
	}

	return scTypes.NewProposalData(
		proposalCandidate.Id,
		proposalCandidate.DataProofs,
		currentProvedStateRoot,
		latestMainRef.Hash,
		proposalCandidate.CreatedAt,
	), nil
}

func (bs *BlockStorage) SetBatchAsProposed(ctx context.Context, id scTypes.BatchId) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.setBatchAsProposedImpl(ctx, id)
	})
}

func (bs *BlockStorage) setBatchAsProposedImpl(ctx context.Context, id scTypes.BatchId) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	batch, err := bs.ops.getBatchEntry(tx, id)
	if err != nil {
		return err
	}

	if !batch.IsProved {
		return fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotProved, id)
	}

	latestMainRef := batch.LatestRefs.TryGetMain()
	if latestMainRef == nil {
		return fmt.Errorf("batch with id=%s has no latest main block", id)
	}

	currentStateRoot, err := bs.ops.getProvedStateRoot(tx)
	switch {
	case err != nil:
		return err
	case currentStateRoot == nil:
		return scTypes.ErrLocalStateRootNotInitialized
	case batch.ParentRefs[types.MainShardId].Hash != *currentStateRoot:
		return fmt.Errorf(
			"%w: currentStateRoot=%s, batch.LatestMain=%s, id=%s",
			scTypes.ErrBatchMismatch, currentStateRoot, latestMainRef, id,
		)
	}

	if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	if err := bs.ops.putProvedStateRoot(tx, latestMainRef.Hash); err != nil {
		return err
	}

	return bs.commit(tx)
}

// ResetBatchesRange resets the block storage state starting from the batch with given ID:
//
//  1. Picks the first main shard block [B] from the batch with the given ID.
//
//  2. Sets the latest fetched block reference to the parent of the block [B].
//     If the specified block is the first block in the chain, the new latest fetched value will be nil.
//
//  3. Deletes all main and corresponding exec shard blocks starting from the block [B].
func (bs *BlockStorage) ResetBatchesRange(
	ctx context.Context,
	firstBatchToPurge scTypes.BatchId,
) (purgedBatches []scTypes.BatchId, err error) {
	err = bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		purgedBatches, err = bs.resetBatchesPartialImpl(ctx, firstBatchToPurge)
		return err
	})
	return
}

func (bs *BlockStorage) resetBatchesPartialImpl(
	ctx context.Context,
	firstBatchToPurge scTypes.BatchId,
) (purgedBatches []scTypes.BatchId, err error) {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := bs.ops.getBatchEntry(tx, firstBatchToPurge); err != nil {
		return nil, err
	}

	latestBatch, err := bs.ops.getLatestBatchId(tx)
	if err != nil {
		return nil, err
	}
	if latestBatch == nil {
		bs.logger.Debug().Msg("no batches created, nothing to purge")
		return nil, nil
	}

	for batch, err := range bs.ops.getBatchesSeqReversed(tx, *latestBatch, firstBatchToPurge) {
		if err != nil {
			return nil, err
		}

		if err := bs.unsetBlockBatch(tx, batch); err != nil {
			return nil, fmt.Errorf("failed to unset batch %s: %w", batch.Id, err)
		}

		purgedBatches = append(purgedBatches, batch.Id)
	}

	if err := bs.commit(tx); err != nil {
		return nil, err
	}

	slices.Reverse(purgedBatches)
	return purgedBatches, nil
}

// unsetBlockBatch removes a batch from storage and resets all related state to the parent batch.
// This operation completely rolls back the effects of putBlockBatch.
//
// In the following example, the storage returns to its initial state:
// putBlockBatch(A) -> putBlockBatch(B) -> unsetBlockBatch(B) -> unsetBlockBatch(A)
func (bs *BlockStorage) unsetBlockBatch(tx db.RwTx, batch *batchEntry) error {
	if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	if batch.ParentId == nil {
		if err := bs.ops.resetLatestFetched(tx); err != nil {
			return err
		}
	} else {
		for shardId, ref := range batch.ParentRefs {
			if err := bs.ops.putLatestFetchedRef(tx, shardId, ref); err != nil {
				return err
			}
		}
	}

	return bs.ops.putLatestBatchId(tx, batch.ParentId)
}

// ResetAllBatches resets the block storage state:
//
//  1. Sets the latest fetched block reference to nil.
//
//  2. Deletes all main blocks from the storage.
func (bs *BlockStorage) ResetAllBatches(ctx context.Context) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.resetAllBatchesImpl(ctx)
	})
}

func (bs *BlockStorage) resetAllBatchesImpl(ctx context.Context) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := bs.ops.resetLatestFetched(tx); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}

	if err := bs.ops.putLatestBatchId(tx, nil); err != nil {
		return fmt.Errorf("failed to reset latest batch id: %w", err)
	}

	if err := bs.deleteBatches(tx, func(batch *batchEntry) bool { return false }); err != nil {
		return fmt.Errorf("failed to delete all batches: %w", err)
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) deleteBatches(tx db.RwTx, skipFilter func(batch *batchEntry) bool) error {
	for batch, err := range bs.ops.getStoredBatchesSeq(tx) {
		if err != nil {
			return err
		}
		if skipFilter(batch) {
			continue
		}

		if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (bs *BlockStorage) putBatchWithBlocks(tx db.RwTx, batch *scTypes.BlockBatch) error {
	currentTime := bs.clock.Now()

	entry := newBatchEntry(batch)
	if err := bs.ops.putBatchEntry(tx, entry); err != nil {
		return err
	}

	for block := range batch.BlocksIter() {
		bEntry := newBlockEntry(block, batch, currentTime)
		if err := bs.ops.putBlockIfNotExist(tx, bEntry, bs.logger); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BlockStorage) deleteBatchWithBlocks(tx db.RwTx, batch *batchEntry) error {
	if err := bs.ops.addStoredCount(tx, -1, bs.config); err != nil {
		return err
	}

	if err := bs.ops.deleteBatch(tx, batch); err != nil {
		return err
	}

	for _, blockId := range batch.BlockIds {
		if err := bs.ops.deleteBlock(tx, blockId, bs.logger); err != nil {
			return err
		}
	}

	return nil
}
