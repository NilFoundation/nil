package storage

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	// blocksTable stores blocks received from the RPC.
	// Key: scTypes.BlockId (block's own id), Value: blockEntry.
	blocksTable db.TableName = "blocks"

	// batchesTable stores blocks batches produced by the Sync Committee.
	// Key: scTypes.BatchId, Value: batchEntry.
	batchesTable db.TableName = "batches"

	// batchParentIdxTable is used for indexing batches by their parent ids.
	// Key: scTypes.BatchId (batch's parent id), Value: scTypes.BatchId (batch's own id);
	batchParentIdxTable db.TableName = "blocks_parent_hash_idx"

	// latestFetchedTable stores reference to the latest main shard block.
	// Key: mainShardKey, Value: scTypes.MainBlockRef.
	latestFetchedTable db.TableName = "latest_fetched"

	// latestBatchIdTable stores identifier of the latest saved batch.
	// Key: mainShardKey, Value: scTypes.BatchId.
	latestBatchIdTable db.TableName = "latest_batch_id"

	// stateRootTable stores the latest ProvedStateRoot (single value).
	// Key: mainShardKey, Value: common.Hash.
	stateRootTable db.TableName = "state_root"

	// nextToProposeTable stores parent's hash of the next block to propose (single value).
	// Key: mainShardKey, Value: common.Hash.
	nextToProposeTable db.TableName = "next_to_propose_parent_hash"

	// storedBatchesCountTable stores the count of batches that have been persisted in the database.
	// Key: mainShardKey, Value: uint32.
	storedBatchesCountTable db.TableName = "stored_batches_count"
)

var mainShardKey = makeShardKey(types.MainShardId)

type batchEntry struct {
	Id                  scTypes.BatchId  `json:"batchId"`
	ParentId            *scTypes.BatchId `json:"parentBatchId,omitempty"`
	MainParentBlockHash common.Hash      `json:"mainParentHash"`

	MainBlockId  scTypes.BlockId   `json:"mainBlockId"`
	ExecBlockIds []scTypes.BlockId `json:"execBlockIds"`

	IsProved  bool      `json:"isProved,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func newBatchEntry(batch *scTypes.BlockBatch, createdAt time.Time) *batchEntry {
	execBlockIds := make([]scTypes.BlockId, 0, len(batch.ChildBlocks))
	for _, childBlock := range batch.ChildBlocks {
		execBlockIds = append(execBlockIds, scTypes.IdFromBlock(childBlock))
	}

	return &batchEntry{
		Id:                  batch.Id,
		ParentId:            batch.ParentId,
		MainParentBlockHash: batch.MainShardBlock.ParentHash,
		MainBlockId:         scTypes.IdFromBlock(batch.MainShardBlock),
		ExecBlockIds:        execBlockIds,
		CreatedAt:           createdAt,
	}
}

type blockEntry struct {
	Block     jsonrpc.RPCBlock `json:"block"`
	BatchId   scTypes.BatchId  `json:"batchId"`
	FetchedAt time.Time        `json:"fetchedAt"`
}

func newBlockEntry(block *jsonrpc.RPCBlock, containingBatch *scTypes.BlockBatch, fetchedAt time.Time) *blockEntry {
	return &blockEntry{
		Block:     *block,
		BatchId:   containingBatch.Id,
		FetchedAt: fetchedAt,
	}
}

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

	return bs.getProvedStateRoot(tx)
}

func (bs *BlockStorage) getProvedStateRoot(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(stateRootTable, mainShardKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
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

	err = tx.Put(stateRootTable, mainShardKey, stateRoot.Bytes())
	if err != nil {
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
	return bs.getLatestBatchIdTx(tx)
}

func (bs *BlockStorage) getLatestBatchIdTx(tx db.RoTx) (*scTypes.BatchId, error) {
	bytes, err := tx.Get(latestBatchIdTable, mainShardKey)

	switch {
	case err == nil:
		break
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	case errors.Is(err, context.Canceled):
		return nil, err
	default:
		return nil, fmt.Errorf("failed to get latest batch id: %w", err)
	}

	if bytes == nil {
		return nil, nil
	}

	var batchId scTypes.BatchId
	if err := batchId.UnmarshalText(bytes); err != nil {
		return nil, err
	}
	return &batchId, nil
}

func (bs *BlockStorage) putLatestBatchIdTx(tx db.RwTx, batchId *scTypes.BatchId) error {
	var bytes []byte

	if batchId != nil {
		var err error
		bytes, err = batchId.MarshalText()
		if err != nil {
			return err
		}
	}

	err := tx.Put(latestBatchIdTable, mainShardKey, bytes)

	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return err
	default:
		return fmt.Errorf("failed to put latest batch id: %w", err)
	}
}

func (bs *BlockStorage) TryGetLatestFetched(ctx context.Context) (*scTypes.MainBlockRef, error) {
	tx, err := bs.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	lastFetched, err := bs.getLatestFetchedMainTx(tx)
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

	entry, err := bs.getBlockEntry(tx, id, false)
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

	if err := bs.putBatchWithBlocksTx(tx, batch); err != nil {
		return err
	}

	if err := bs.setProposeParentHash(tx, batch.MainShardBlock); err != nil {
		return err
	}

	if err := bs.updateLatestFetched(tx, batch.MainShardBlock); err != nil {
		return err
	}

	latestBatchId, err := bs.getLatestBatchIdTx(tx)
	if err != nil {
		return err
	}
	if err := bs.validateLatestBatchId(batch, latestBatchId); err != nil {
		return err
	}

	if err := bs.putLatestBatchIdTx(tx, &batch.Id); err != nil {
		return err
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) validateLatestBatchId(batch *scTypes.BlockBatch, latestBatchId *scTypes.BatchId) error {
	var isValid bool
	switch {
	case latestBatchId == nil:
		isValid = batch.ParentId == nil
	case batch.ParentId == nil:
		isValid = false
	default:
		isValid = *latestBatchId == *batch.ParentId
	}

	if isValid {
		return nil
	}

	return fmt.Errorf(
		"%w: got batch with parentId=%s, latest batch id is %s",
		scTypes.ErrBatchMismatch, batch.ParentId, latestBatchId,
	)
}

func (bs *BlockStorage) updateLatestFetched(tx db.RwTx, block *jsonrpc.RPCBlock) error {
	if block.ShardId != types.MainShardId {
		return nil
	}

	latestFetched, err := bs.getLatestFetchedMainTx(tx)
	if err != nil {
		return err
	}

	if latestFetched.Equals(block) {
		return nil
	}

	if err := latestFetched.ValidateChild(block); err != nil {
		return fmt.Errorf("unable to update latest fetched block: %w", err)
	}

	newLatestFetched, err := scTypes.NewBlockRef(block)
	if err != nil {
		return err
	}

	return bs.putLatestFetchedBlockTx(tx, block.ShardId, newLatestFetched)
}

func (bs *BlockStorage) setProposeParentHash(tx db.RwTx, block *jsonrpc.RPCBlock) error {
	if block.ShardId != types.MainShardId {
		return nil
	}
	parentHash, err := bs.getParentOfNextToPropose(tx)
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

	return bs.setParentOfNextToPropose(tx, block.ParentHash)
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

	entry, err := bs.getBatchTx(tx, batchId)
	if err != nil {
		return false, err
	}

	if entry.IsProved {
		bs.logger.Debug().Stringer(logging.FieldBatchId, batchId).Msg("batch is already marked as proved")
		return false, nil
	}

	entry.IsProved = true
	if err := bs.putBatchTx(tx, entry); err != nil {
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

	currentProvedStateRoot, err := bs.getProvedStateRoot(tx)
	if err != nil {
		return nil, err
	}
	if currentProvedStateRoot == nil {
		return nil, ErrStateRootNotInitialized
	}

	parentHash, err := bs.getParentOfNextToPropose(tx)
	if err != nil {
		return nil, err
	}

	if parentHash == nil {
		bs.logger.Debug().Msg("block parent hash is not set")
		return nil, nil
	}

	var proposalCandidate *batchEntry
	for entry, err := range bs.getStoredBatchesSeq(tx) {
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
	mainBlockEntry, err := bs.getBlockEntry(tx, proposalCandidate.MainBlockId, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get main block with id=%s: %w", proposalCandidate.MainBlockId, err)
	}

	transactions := scTypes.BlockTransactions(&mainBlockEntry.Block)

	for _, childId := range proposalCandidate.ExecBlockIds {
		childEntry, err := bs.getBlockEntry(tx, childId, true)
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

	batch, err := bs.getBatchTx(tx, id)
	if err != nil {
		return err
	}

	if !batch.IsProved {
		return fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotProved, id)
	}

	mainShardEntry, err := bs.getBlockEntry(tx, batch.MainBlockId, true)
	if err != nil {
		return err
	}

	if err := bs.validateMainShardEntry(tx, mainShardEntry); err != nil {
		return err
	}

	if err := bs.deleteBatchWithBlocksTx(tx, batch); err != nil {
		return err
	}

	if err := tx.Put(stateRootTable, mainShardKey, mainShardEntry.Block.Hash.Bytes()); err != nil {
		return fmt.Errorf("failed to put state root: %w", err)
	}

	if err := bs.setParentOfNextToPropose(tx, mainShardEntry.Block.Hash); err != nil {
		return err
	}

	return bs.commit(tx)
}

func isValidProposalCandidate(batch *batchEntry, parentHash common.Hash) bool {
	return batch.IsProved && batch.MainParentBlockHash == parentHash
}

// getParentOfNextToPropose retrieves parent's hash of the next block to propose
func (bs *BlockStorage) getParentOfNextToPropose(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(nextToProposeTable, mainShardKey)

	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get next to propose parent hash: %w", err)
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

// setParentOfNextToPropose sets parent's hash of the next block to propose
func (bs *BlockStorage) setParentOfNextToPropose(tx db.RwTx, hash common.Hash) error {
	err := tx.Put(nextToProposeTable, mainShardKey, hash.Bytes())
	if err != nil {
		return fmt.Errorf("failed to put next to propose parent hash: %w", err)
	}
	return nil
}

func (bs *BlockStorage) validateMainShardEntry(tx db.RoTx, entry *blockEntry) error {
	id := scTypes.IdFromBlock(&entry.Block)

	if entry.Block.ShardId != types.MainShardId {
		return fmt.Errorf("block with id=%s is not from main shard", id.String())
	}

	parentHash, err := bs.getParentOfNextToPropose(tx)
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

func (bs *BlockStorage) getLatestFetchedMainTx(tx db.RoTx) (*scTypes.MainBlockRef, error) {
	value, err := tx.Get(latestFetchedTable, mainShardKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var blockRef *scTypes.MainBlockRef
	err = json.Unmarshal(value, &blockRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSerializationFailed, err)
	}
	return blockRef, nil
}

func (bs *BlockStorage) putLatestFetchedBlockTx(tx db.RwTx, shardId types.ShardId, block *scTypes.MainBlockRef) error {
	bytes, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf(
			"%w: failed to encode block ref with hash=%s: %w", ErrSerializationFailed, block.Hash.String(), err,
		)
	}
	err = tx.Put(latestFetchedTable, makeShardKey(shardId), bytes)
	if err != nil {
		return fmt.Errorf("failed to put block ref with hash=%s: %w", block.Hash.String(), err)
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

	startingBatch, err := bs.getBatchTx(tx, firstBatchToPurge)
	if err != nil {
		return nil, err
	}

	if err := bs.resetToParent(tx, startingBatch); err != nil {
		return nil, err
	}

	for batch, err := range bs.getBatchesSequence(tx, firstBatchToPurge) {
		if err != nil {
			return nil, err
		}

		if err := bs.deleteBatchWithBlocksTx(tx, batch); err != nil {
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
	mainBlockEntry, err := bs.getBlockEntry(tx, batch.MainBlockId, true)
	if err != nil {
		return err
	}

	refToParent, err := scTypes.GetMainParentRef(&mainBlockEntry.Block)
	if err != nil {
		return fmt.Errorf("failed to get main block parent ref: %w", err)
	}
	if err := bs.putLatestFetchedBlockTx(tx, types.MainShardId, refToParent); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}
	if err := bs.putLatestBatchIdTx(tx, batch.ParentId); err != nil {
		return fmt.Errorf("failed to reset latest batch id: %w", err)
	}

	return nil
}

// getBatchesSequence iterates through a chain of batches, starting from the batch with the given id.
// It uses batchParentIdxTable to retrieve parent-child connections between batches.
func (bs *BlockStorage) getBatchesSequence(tx db.RoTx, startingId scTypes.BatchId) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		startBatch, err := bs.getBatchTx(tx, startingId)
		if err != nil {
			yield(nil, err)
			return
		}

		if !yield(startBatch, nil) {
			return
		}

		seenParentIds := make(map[scTypes.BatchId]bool)
		nextParentId := startBatch.Id
		for {
			if seenParentIds[nextParentId] {
				yield(nil, fmt.Errorf("cycle detected in the batch chain, parentId=%s", nextParentId))
				return
			}
			seenParentIds[nextParentId] = true

			nextIdBytes, err := tx.Get(batchParentIdxTable, nextParentId.Bytes())
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				yield(nil, fmt.Errorf("failed to get parent batch idx entry, parentId=%s: %w", nextParentId, err))
				return
			}
			if nextIdBytes == nil {
				break
			}
			nextBatchEntry, err := bs.getBatchBytesIdTx(tx, nextIdBytes, true)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(nextBatchEntry, nil) {
				return
			}
			nextParentId = nextBatchEntry.Id
		}
	}
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

	if err := bs.putLatestFetchedBlockTx(tx, types.MainShardId, nil); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}

	for batch, err := range bs.getStoredBatchesSeq(tx) {
		if err != nil {
			return err
		}
		if batch.IsProved {
			continue
		}

		if err := bs.deleteBatchWithBlocksTx(tx, batch); err != nil {
			return err
		}
	}

	return bs.commit(tx)
}

func makeShardKey(shardId types.ShardId) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(shardId))
	return key
}

func (bs *BlockStorage) getBlockEntry(tx db.RoTx, id scTypes.BlockId, required bool) (*blockEntry, error) {
	return bs.getBlockEntryBytesId(tx, id.Bytes(), required)
}

func (bs *BlockStorage) getBlockEntryBytesId(tx db.RoTx, idBytes []byte, required bool) (*blockEntry, error) {
	value, err := tx.Get(blocksTable, idBytes)

	switch {
	case err == nil:
		break
	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBlockNotFound, hex.EncodeToString(idBytes))
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to get block with id=%s: %w", hex.EncodeToString(idBytes), err)
	}

	entry, err := unmarshallEntry[blockEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (bs *BlockStorage) putBatchWithBlocksTx(tx db.RwTx, batch *scTypes.BlockBatch) error {
	if err := bs.addStoredCountTx(tx, 1); err != nil {
		return err
	}

	if err := bs.putBatchParentIndexEntryTx(tx, batch); err != nil {
		return err
	}

	currentTime := bs.timer.NowTime()

	entry := newBatchEntry(batch, currentTime)
	if err := bs.putBatchTx(tx, entry); err != nil {
		return err
	}

	mainEntry := newBlockEntry(batch.MainShardBlock, batch, currentTime)
	if err := bs.putBlockTx(tx, mainEntry); err != nil {
		return err
	}

	for _, childBlock := range batch.ChildBlocks {
		childEntry := newBlockEntry(childBlock, batch, currentTime)
		if err := bs.putBlockTx(tx, childEntry); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BlockStorage) putBatchParentIndexEntryTx(tx db.RwTx, batch *scTypes.BlockBatch) error {
	if batch.ParentId == nil {
		return nil
	}

	err := tx.Put(batchParentIdxTable, batch.ParentId.Bytes(), batch.Id.Bytes())
	if err != nil {
		return fmt.Errorf(
			"failed to put parent batch idx entry, batchId=%s, parentId=%s,: %w", batch.Id, batch.ParentId, err,
		)
	}

	return nil
}

func (bs *BlockStorage) deleteBatchParentIndexEntryTx(tx db.RwTx, batch *batchEntry) error {
	if batch.ParentId == nil {
		return nil
	}

	err := tx.Delete(batchParentIdxTable, batch.ParentId.Bytes())

	switch {
	case err == nil:
		return nil

	case errors.Is(err, context.Canceled):
		return err

	case errors.Is(err, db.ErrKeyNotFound):
		bs.logger.Warn().Err(err).
			Stringer(logging.FieldBatchId, batch.Id).
			Stringer("parentBatchId", batch.ParentId).
			Msg("parent batch idx entry is not found")
		return nil

	default:
		return fmt.Errorf("failed to delete parent batch idx entry, parentId=%s: %w", batch.ParentId, err)
	}
}

func (bs *BlockStorage) putBatchTx(tx db.RwTx, entry *batchEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, id=%s", err, entry.Id)
	}

	if err := tx.Put(batchesTable, entry.Id.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put batch with id=%s: %w", entry.Id, err)
	}

	return nil
}

func (bs *BlockStorage) getBatchTx(tx db.RoTx, id scTypes.BatchId) (*batchEntry, error) {
	return bs.getBatchBytesIdTx(tx, id.Bytes(), true)
}

func (bs *BlockStorage) getBatchBytesIdTx(tx db.RoTx, idBytes []byte, required bool) (*batchEntry, error) {
	value, err := tx.Get(batchesTable, idBytes)

	switch {
	case err == nil:
		break

	case errors.Is(err, context.Canceled):
		return nil, err

	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotFound, hex.EncodeToString(idBytes))

	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil

	default:
		return nil, fmt.Errorf("failed to get batch with id=%s: %w", hex.EncodeToString(idBytes), err)
	}

	entry, err := unmarshallEntry[batchEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (bs *BlockStorage) deleteBatchWithBlocksTx(tx db.RwTx, batch *batchEntry) error {
	if err := bs.addStoredCountTx(tx, -1); err != nil {
		return err
	}

	if err := tx.Delete(batchesTable, batch.Id.Bytes()); err != nil {
		return fmt.Errorf("failed to delete batch with id=%s: %w", batch.Id, err)
	}

	if err := bs.deleteBatchParentIndexEntryTx(tx, batch); err != nil {
		return err
	}

	if err := bs.deleteBlock(tx, batch.MainBlockId); err != nil {
		return err
	}

	for _, childId := range batch.ExecBlockIds {
		if err := bs.deleteBlock(tx, childId); err != nil {
			return err
		}
	}

	return nil
}

func (bs *BlockStorage) putBlockTx(tx db.RwTx, entry *blockEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, hash=%s", err, entry.Block.Hash)
	}

	blockId := scTypes.IdFromBlock(&entry.Block)
	if err := tx.Put(blocksTable, blockId.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put block %s: %w", blockId.String(), err)
	}

	return nil
}

func (bs *BlockStorage) deleteBlock(tx db.RwTx, blockId scTypes.BlockId) error {
	err := tx.Delete(blocksTable, blockId.Bytes())

	switch {
	case err == nil:
		return nil

	case errors.Is(err, context.Canceled):
		return err

	case errors.Is(err, db.ErrKeyNotFound):
		bs.logger.Warn().Err(err).
			Stringer(logging.FieldShardId, blockId.ShardId).
			Stringer(logging.FieldBlockHash, blockId.Hash).
			Msg("block is not found (deleteBlock)")
		return nil

	default:
		return fmt.Errorf("failed to delete block with id=%s: %w", blockId, err)
	}
}

// getStoredBatchesSeq returns a sequence of stored batches in an arbitrary order.
func (*BlockStorage) getStoredBatchesSeq(tx db.RoTx) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		txIter, err := tx.Range(batchesTable, nil, nil)
		if err != nil {
			yield(nil, err)
			return
		}
		defer txIter.Close()

		for txIter.HasNext() {
			key, val, err := txIter.Next()
			if err != nil {
				yield(nil, err)
				return
			}
			entry, err := unmarshallEntry[batchEntry](key, val)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (bs *BlockStorage) addStoredCountTx(tx db.RwTx, delta int32) error {
	currentBatchesCount, err := bs.getBatchesCountTx(tx)
	if err != nil {
		return err
	}

	signed := int32(currentBatchesCount) + delta
	if signed < 0 {
		return fmt.Errorf(
			"batches count cannot be negative: delta=%d, current blocks count=%d", delta, currentBatchesCount,
		)
	}

	newBatchesCount := uint32(signed)
	if newBatchesCount > bs.config.StoredBatchesLimit {
		return fmt.Errorf(
			"%w: delta is %d, current storage size is %d, capacity limit is %d",
			ErrCapacityLimitReached, delta, currentBatchesCount, bs.config.StoredBatchesLimit,
		)
	}

	return bs.putBatchesCountTx(tx, newBatchesCount)
}

func (bs *BlockStorage) getBatchesCountTx(tx db.RoTx) (uint32, error) {
	bytes, err := tx.Get(storedBatchesCountTable, mainShardKey)
	switch {
	case err == nil:
		break
	case errors.Is(err, db.ErrKeyNotFound):
		return 0, nil
	default:
		return 0, fmt.Errorf("failed to get batches count: %w", err)
	}

	count := binary.LittleEndian.Uint32(bytes)
	return count, nil
}

func (bs *BlockStorage) putBatchesCountTx(tx db.RwTx, newValue uint32) error {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, newValue)

	err := tx.Put(storedBatchesCountTable, mainShardKey, bytes)
	if err != nil {
		return fmt.Errorf("failed to put batches count: %w (newValue is %d)", err, newValue)
	}
	return nil
}

func marshallEntry[E any](entry *E) ([]byte, error) {
	bytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: failed to marshall entry: %w", ErrSerializationFailed, err,
		)
	}
	return bytes, nil
}

func unmarshallEntry[E any](key []byte, val []byte) (*E, error) {
	entry := new(E)

	if err := json.Unmarshal(val, entry); err != nil {
		return nil, fmt.Errorf(
			"%w: failed to unmarshall entry with id=%s: %w", ErrSerializationFailed, hex.EncodeToString(key), err,
		)
	}

	return entry, nil
}
