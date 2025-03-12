package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
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
	timer   common.Timer
	metrics BlockStorageMetrics
}

func NewBlockStorage(
	database db.DB,
	config BlockStorageConfig,
	timer common.Timer,
	metrics BlockStorageMetrics,
	logger zerolog.Logger,
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
		timer:   timer,
		metrics: metrics,
	}
}

func (bs *BlockStorage) TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return newStateRootOp().getProvedStateRoot(tx)
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

	if err := newStateRootOp().putProvedStateRoot(tx, stateRoot); err != nil {
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
	return newLatestBatchOp().getLatestBatchId(tx)
}

func (bs *BlockStorage) TryGetLatestFetched(ctx context.Context) (*scTypes.MainBlockRef, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	lastFetched, err := newBlocksLatestFetchedOp().getLatestFetchedMain(tx)
	if err != nil {
		return nil, err
	}

	return lastFetched, nil
}

func (bs *BlockStorage) TryGetBlock(ctx context.Context, id scTypes.BlockId) (*jsonrpc.RPCBlock, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entry, err := newBlockOp(bs.logger).getBlock(tx, id, false)
	if err != nil || entry == nil {
		return nil, err
	}
	return &entry.Block, nil
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

	if err := bs.putBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	if err := bs.setProposeParentHash(tx, batch.MainShardBlock); err != nil {
		return err
	}

	if err := newBlocksLatestFetchedOp().updateLatestFetched(tx, batch.MainShardBlock); err != nil {
		return err
	}

	if err := newLatestBatchOp().updateLatestBatchId(tx, batch); err != nil {
		return err
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) setProposeParentHash(tx db.RwTx, block *jsonrpc.RPCBlock) error {
	rootOp := newStateRootOp()

	if block.ShardId != types.MainShardId {
		return nil
	}
	parentHash, err := rootOp.getParentOfNextToPropose(tx)
	if err != nil {
		return err
	}
	if parentHash != nil {
		return nil
	}

	if block.Number > 0 && block.ParentHash.Empty() {
		return fmt.Errorf("block with hash=%s has empty parent hash", block.Hash.String())
	}

	bs.logger.Info().
		Stringer(logging.FieldBlockHash, block.Hash).
		Stringer("parentHash", block.ParentHash).
		Msg("block parent hash is not set, updating it")

	return rootOp.setParentOfNextToPropose(tx, block.ParentHash)
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
	batchesOp := newBatchOp(bs.logger)

	entry, err := batchesOp.getBatch(tx, batchId)
	if err != nil {
		return false, err
	}

	if entry.IsProved {
		bs.logger.Debug().Stringer(logging.FieldBatchId, batchId).Msg("batch is already marked as proved")
		return false, nil
	}

	entry.IsProved = true
	if err := batchesOp.putBatch(tx, entry); err != nil {
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
	rootOp := newStateRootOp()

	currentProvedStateRoot, err := rootOp.getProvedStateRoot(tx)
	if err != nil {
		return nil, err
	}
	if currentProvedStateRoot == nil {
		return nil, ErrStateRootNotInitialized
	}

	parentHash, err := rootOp.getParentOfNextToPropose(tx)
	if err != nil {
		return nil, err
	}

	if parentHash == nil {
		bs.logger.Debug().Msg("block parent hash is not set")
		return nil, nil
	}

	var proposalCandidate *batchEntry
	for entry, err := range newBatchOp(bs.logger).getStoredBatchesSeq(tx) {
		if err != nil {
			return nil, err
		}
		if isValidProposalCandidate(entry, *parentHash) {
			proposalCandidate = entry
			break
		}
	}

	if proposalCandidate == nil {
		bs.logger.Debug().Stringer("parentHash", parentHash).Msg("no proved batch found")
		return nil, nil
	}

	return bs.createProposalDataTx(tx, proposalCandidate, currentProvedStateRoot)
}

func (bs *BlockStorage) createProposalDataTx(
	tx db.RoTx,
	proposalCandidate *batchEntry,
	currentProvedStateRoot *common.Hash,
) (*scTypes.ProposalData, error) {
	blocksOp := newBlockOp(bs.logger)

	mainBlockEntry, err := blocksOp.getBlock(tx, proposalCandidate.MainBlockId, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get main block with id=%s: %w", proposalCandidate.MainBlockId, err)
	}

	transactions := scTypes.BlockTransactions(&mainBlockEntry.Block)

	for _, childId := range proposalCandidate.ExecBlockIds {
		childEntry, err := blocksOp.getBlock(tx, childId, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get child block with id=%s: %w", childId, err)
		}

		blockTransactions := scTypes.BlockTransactions(&childEntry.Block)
		transactions = append(transactions, blockTransactions...)
	}

	return &scTypes.ProposalData{
		BatchId:            proposalCandidate.Id,
		MainShardBlockHash: mainBlockEntry.Block.Hash,
		Transactions:       transactions,
		OldProvedStateRoot: *currentProvedStateRoot,
		NewProvedStateRoot: mainBlockEntry.Block.Hash,
		MainBlockFetchedAt: mainBlockEntry.FetchedAt,
	}, nil
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
	batchesOp := newBatchOp(bs.logger)

	batch, err := batchesOp.getBatch(tx, id)
	if err != nil {
		return err
	}

	if !batch.IsProved {
		return fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotProved, id)
	}

	mainShardEntry, err := newBlockOp(bs.logger).getBlock(tx, batch.MainBlockId, true)
	if err != nil {
		return err
	}

	if err := bs.validateMainShardEntry(tx, mainShardEntry); err != nil {
		return err
	}

	if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
		return err
	}

	rootOp := newStateRootOp()
	if err := rootOp.putProvedStateRoot(tx, mainShardEntry.Block.Hash); err != nil {
		return err
	}
	if err := rootOp.setParentOfNextToPropose(tx, mainShardEntry.Block.Hash); err != nil {
		return err
	}

	return bs.commit(tx)
}

func isValidProposalCandidate(batch *batchEntry, parentHash common.Hash) bool {
	return batch.IsProved && batch.MainParentBlockHash == parentHash
}

func (bs *BlockStorage) validateMainShardEntry(tx db.RoTx, entry *blockEntry) error {
	id := scTypes.IdFromBlock(&entry.Block)

	if entry.Block.ShardId != types.MainShardId {
		return fmt.Errorf("block with id=%s is not from main shard", id.String())
	}

	parentHash, err := newStateRootOp().getParentOfNextToPropose(tx)
	if err != nil {
		return err
	}
	if parentHash == nil {
		return errors.New("next to propose parent hash is not set")
	}

	if *parentHash != entry.Block.ParentHash {
		return fmt.Errorf(
			"parent's block hash=%s is not equal to the stored value=%s",
			entry.Block.ParentHash.String(),
			parentHash.String(),
		)
	}
	return nil
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
	batchesOp := newBatchOp(bs.logger)

	startingBatch, err := batchesOp.getBatch(tx, firstBatchToPurge)
	if err != nil {
		return nil, err
	}

	if err := bs.resetToParent(tx, startingBatch); err != nil {
		return nil, err
	}

	for batch, err := range batchesOp.getBatchesSequence(tx, firstBatchToPurge) {
		if err != nil {
			return nil, err
		}

		if err := bs.deleteBatchWithBlocks(tx, batch); err != nil {
			return nil, err
		}

		purgedBatches = append(purgedBatches, batch.Id)
	}

	if err := bs.commit(tx); err != nil {
		return nil, err
	}

	return purgedBatches, nil
}

func (bs *BlockStorage) resetToParent(tx db.RwTx, batch *batchEntry) error {
	mainBlockEntry, err := newBlockOp(bs.logger).getBlock(tx, batch.MainBlockId, true)
	if err != nil {
		return err
	}

	refToParent, err := scTypes.GetMainParentRef(&mainBlockEntry.Block)
	if err != nil {
		return fmt.Errorf("failed to get main block parent ref: %w", err)
	}
	if err := newBlocksLatestFetchedOp().putLatestFetchedBlock(tx, types.MainShardId, refToParent); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}
	if err := newLatestBatchOp().putLatestBatchId(tx, batch.ParentId); err != nil {
		return fmt.Errorf("failed to reset latest batch id: %w", err)
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

	if err := newBlocksLatestFetchedOp().putLatestFetchedBlock(tx, types.MainShardId, nil); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}

	for batch, err := range newBatchOp(bs.logger).getStoredBatchesSeq(tx) {
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
	batchesOp := newBatchOp(bs.logger)

	if err := newBatchCountOp(bs.config).addStoredCount(tx, 1); err != nil {
		return err
	}

	if err := batchesOp.putBatchParentIndexEntry(tx, batch); err != nil {
		return err
	}

	currentTime := bs.timer.NowTime()

	entry := newBatchEntry(batch, currentTime)
	if err := batchesOp.putBatch(tx, entry); err != nil {
		return err
	}

	blocksOp := newBlockOp(bs.logger)

	mainEntry := newBlockEntry(batch.MainShardBlock, batch, currentTime)
	if err := blocksOp.putBlockTx(tx, mainEntry); err != nil {
		return err
	}

	for _, childBlock := range batch.ChildBlocks {
		childEntry := newBlockEntry(childBlock, batch, currentTime)
		if err := blocksOp.putBlockTx(tx, childEntry); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BlockStorage) deleteBatchWithBlocks(tx db.RwTx, batch *batchEntry) error {
	if err := newBatchCountOp(bs.config).addStoredCount(tx, -1); err != nil {
		return err
	}

	if err := newBatchOp(bs.logger).deleteBatch(tx, batch); err != nil {
		return err
	}

	blocksOp := newBlockOp(bs.logger)

	if err := blocksOp.deleteBlock(tx, batch.MainBlockId); err != nil {
		return err
	}

	for _, childId := range batch.ExecBlockIds {
		if err := blocksOp.deleteBlock(tx, childId); err != nil {
			return err
		}
	}

	return nil
}
