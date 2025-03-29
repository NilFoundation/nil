package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

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
				scTypes.ErrBatchMismatch, scTypes.ErrBlockNotFound, scTypes.ErrBatchNotFound, scTypes.ErrBatchNotProved,
				ErrStateRootNotInitialized,
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

// TryGetLatestBatchId retrieves the ID of the latest created batch
// or returns nil if:
// a) No batches have been created yet, or
// b) A full storage reset (starting from the first batch) has been triggered.
func (bs *BlockStorage) TryGetLatestBatchId(ctx context.Context) (*scTypes.BatchId, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return bs.ops.getLatestBatchId(tx)
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

func (bs *BlockStorage) GetFreeSpaceBatchCount(ctx context.Context) (uint32, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	batchCount, err := bs.ops.getBatchesCount(tx)
	if err != nil {
		return 0, err
	}
	return bs.config.StoredBatchesLimit - batchCount, nil
}

func (bs *BlockStorage) SetBlockBatch(ctx context.Context, batch *scTypes.BlockBatch) error {
	if batch == nil {
		return errors.New("batch cannot be nil")
	}

	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.setBlockBatchImpl(ctx, batch)
	})
}

func (bs *BlockStorage) setBlockBatchImpl(ctx context.Context, batch *scTypes.BlockBatch) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := bs.validateBatchSequencing(tx, batch); err != nil {
		return err
	}

	if err := bs.putBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	latestFetched := batch.LatestRefs()
	if err := bs.ops.putLatestFetchedRefs(tx, latestFetched); err != nil {
		return err
	}

	if err := bs.ops.updateLatestBatchId(tx, batch); err != nil {
		return err
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) validateBatchSequencing(tx db.RoTx, batch *scTypes.BlockBatch) error {
	currentLatestRefs, err := bs.ops.getLatestFetched(tx)
	if err != nil {
		return err
	}

	for shard, batchParentRef := range batch.ParentRefs() {
		currentLatestRef := currentLatestRefs.TryGet(shard)

		if currentLatestRef == nil || batchParentRef.Equals(currentLatestRef) {
			continue
		}

		return fmt.Errorf(
			"%w, id=%s: parent ref %s does not match current ref %s",
			scTypes.ErrBatchMismatch, batch.Id, batchParentRef, currentLatestRef,
		)
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

	entry, err := bs.ops.getBatch(tx, batchId)
	if err != nil {
		return false, err
	}

	if entry.IsProved {
		bs.logger.Debug().Stringer(logging.FieldBatchId, batchId).Msg("batch is already marked as proved")
		return false, nil
	}

	entry.IsProved = true
	if err := bs.ops.putBatch(tx, entry); err != nil {
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
		return nil, ErrStateRootNotInitialized
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

	return bs.createProposalDataTx(tx, proposalCandidate, *currentProvedStateRoot)
}

func (bs *BlockStorage) createProposalDataTx(
	tx db.RoTx,
	proposalCandidate *batchEntry,
	currentProvedStateRoot common.Hash,
) (*scTypes.ProposalData, error) {
	transactions := make([]scTypes.PrunedTransaction, 0)

	var firstBlockFetchedAt time.Time

	for i, blockId := range proposalCandidate.BlockIds {
		bEntry, err := bs.ops.getBlock(tx, blockId, true)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			firstBlockFetchedAt = bEntry.FetchedAt
		}

		blockTransactions := scTypes.BlockTransactions(&bEntry.Block)
		transactions = append(transactions, blockTransactions...)
	}

	return scTypes.NewProposalData(
		proposalCandidate.Id,
		transactions,
		currentProvedStateRoot,
		proposalCandidate.LatestMainBlockHash,
		firstBlockFetchedAt,
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

	batch, err := bs.ops.getBatch(tx, id)
	if err != nil {
		return err
	}

	if !batch.IsProved {
		return fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotProved, id)
	}

	currentStateRoot, err := bs.ops.getProvedStateRoot(tx)
	switch {
	case err != nil:
		return err
	case currentStateRoot == nil:
		return ErrStateRootNotInitialized
	case batch.ParentRefs[types.MainShardId].Hash != *currentStateRoot:
		return fmt.Errorf(
			"%w: currentStateRoot=%s, batch.LatestMainBlockHash=%s, id=%s",
			scTypes.ErrBatchMismatch, currentStateRoot, batch.LatestMainBlockHash, id,
		)
	}

	if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	if err := bs.ops.putProvedStateRoot(tx, batch.LatestMainBlockHash); err != nil {
		return err
	}

	return bs.commit(tx)
}

// ResetBatchesRange resets the block storage state starting from the batch with given ID:
//
//  1. Picks first main shard block [B] from the batch with the given ID.
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

	if _, err := bs.ops.getBatch(tx, firstBatchToPurge); err != nil {
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
// This operation completely rolls back the effects of setBlockBatchImpl.
//
// In the following example, the storage returns to its initial state:
// setBlockBatchImpl(A) -> setBlockBatchImpl(B) -> unsetBlockBatch(B) -> unsetBlockBatch(A)
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

	if err := bs.ops.putLatestBatchId(tx, batch.ParentId); err != nil {
		return err
	}

	return nil
}

// ResetBatchesNotProved resets the block storage state:
//
//  1. Sets the latest fetched block reference to nil.
//
//  2. Deletes all main not yet proved blocks from the storage.
func (bs *BlockStorage) ResetBatchesNotProved(ctx context.Context) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.resetBatchesNotProvedImpl(ctx)
	})
}

func (bs *BlockStorage) resetBatchesNotProvedImpl(ctx context.Context) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := bs.ops.resetLatestFetched(tx); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}

	for batch, err := range bs.ops.getStoredBatchesSeq(tx) {
		if err != nil {
			return err
		}
		if batch.IsProved {
			continue
		}

		if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
			return err
		}
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) putBatchWithBlocks(tx db.RwTx, batch *scTypes.BlockBatch) error {
	if err := bs.ops.addStoredCount(tx, 1, bs.config); err != nil {
		return err
	}

	currentTime := bs.clock.Now()

	entry := newBatchEntry(batch, currentTime)
	if err := bs.ops.putBatch(tx, entry); err != nil {
		return err
	}

	for block := range batch.BlocksIter() {
		bEntry := newBlockEntry(block, batch, currentTime)
		if err := bs.ops.putBlockTx(tx, bEntry); err != nil {
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
