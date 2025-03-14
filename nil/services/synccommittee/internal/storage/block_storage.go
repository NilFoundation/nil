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

	// blockParentIdxTable is used for indexing blocks by their parent ids.
	// Key: scTypes.BlockId (block's parent id), Value: scTypes.BlockId (block's own id);
	blockParentIdxTable db.TableName = "blocks_parent_hash_idx"

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

	// storedBlocksCountTable stores the count of blocks that have been persisted in the database.
	// Key: mainShardKey, Value: uint32.
	storedBlocksCountTable db.TableName = "stored_blocks_count"
)

var mainShardKey = makeShardKey(types.MainShardId)

type blockEntry struct {
	Block         jsonrpc.RPCBlock `json:"block"`
	IsProved      bool             `json:"isProved"`
	BatchId       scTypes.BatchId  `json:"batchId"`
	ParentBatchId *scTypes.BatchId `json:"parentBatchId"`
	FetchedAt     time.Time        `json:"fetchedAt"`
}

func newBlockEntry(block *jsonrpc.RPCBlock, containingBatch *scTypes.BlockBatch, fetchedAt time.Time) *blockEntry {
	return &blockEntry{
		Block:         *block,
		BatchId:       containingBatch.Id,
		ParentBatchId: containingBatch.ParentId,
		FetchedAt:     fetchedAt,
	}
}

type BlockStorageMetrics interface {
	RecordMainBlockProved(ctx context.Context)
}

type BlockStorageConfig struct {
	CapacityLimit uint32
}

func NewBlockStorageConfig(capacityLimit uint32) BlockStorageConfig {
	return BlockStorageConfig{
		CapacityLimit: capacityLimit,
	}
}

func DefaultBlockStorageConfig() BlockStorageConfig {
	return NewBlockStorageConfig(200)
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
			common.DoNotRetryIf(scTypes.ErrBlockMismatch, scTypes.ErrBlockNotFound, scTypes.ErrBatchMismatch),
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

	if err := bs.addStoredCountTx(tx, int32(batch.BlocksCount())); err != nil {
		return err
	}

	currentTime := bs.timer.NowTime()
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

func (bs *BlockStorage) SetBlockAsProved(ctx context.Context, id scTypes.BlockId) error {
	wasSet, err := bs.setBlockAsProvedImpl(ctx, id)
	if err != nil {
		return err
	}
	if wasSet {
		bs.metrics.RecordMainBlockProved(ctx)
	}
	return nil
}

func (bs *BlockStorage) setBlockAsProvedImpl(ctx context.Context, id scTypes.BlockId) (wasSet bool, err error) {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	entry, err := bs.getBlockEntry(tx, id, true)
	if err != nil {
		return false, err
	}

	if entry.IsProved {
		bs.logger.Debug().Stringer("blockId", id).Msg("block is already marked as proved")
		return false, nil
	}

	entry.IsProved = true
	value, err := marshallEntry(entry)
	if err != nil {
		return false, err
	}

	if err := tx.Put(blocksTable, id.Bytes(), value); err != nil {
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
		return nil, errors.New("proved state root was not initialized")
	}

	parentHash, err := bs.getParentOfNextToPropose(tx)
	if err != nil {
		return nil, err
	}

	if parentHash == nil {
		bs.logger.Debug().Msg("block parent hash is not set")
		return nil, nil
	}

	var mainShardEntry *blockEntry
	for entry, err := range bs.storedBlocksIter(tx) {
		if err != nil {
			return nil, err
		}
		if isValidProposalCandidate(entry, *parentHash) {
			mainShardEntry = entry
			break
		}
	}

	if mainShardEntry == nil {
		bs.logger.Debug().Stringer("parentHash", parentHash).Msg("no proved main shard block found")
		return nil, nil
	}

	transactions := scTypes.BlockTransactions(&mainShardEntry.Block)

	childIds, err := scTypes.ChildBlockIds(&mainShardEntry.Block)
	if err != nil {
		return nil, err
	}

	for _, childId := range childIds {
		childEntry, err := bs.getBlockEntry(tx, childId, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get child block with id=%s: %w", childId, err)
		}

		blockTransactions := scTypes.BlockTransactions(&childEntry.Block)
		transactions = append(transactions, blockTransactions...)
	}

	return &scTypes.ProposalData{
		MainShardBlockHash: mainShardEntry.Block.Hash,
		Transactions:       transactions,
		OldProvedStateRoot: *currentProvedStateRoot,
		NewProvedStateRoot: mainShardEntry.Block.ChildBlocksRootHash,
		MainBlockFetchedAt: mainShardEntry.FetchedAt,
	}, nil
}

func (bs *BlockStorage) SetBlockAsProposed(ctx context.Context, id scTypes.BlockId) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.setBlockAsProposedImpl(ctx, id)
	})
}

func (bs *BlockStorage) setBlockAsProposedImpl(ctx context.Context, id scTypes.BlockId) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	mainShardEntry, err := bs.getBlockEntry(tx, id, true)
	if err != nil {
		return err
	}

	if err := bs.validateMainShardEntry(tx, id, mainShardEntry); err != nil {
		return err
	}

	if err := bs.deleteMainBlockWithChildren(tx, mainShardEntry); err != nil {
		return err
	}

	if err := tx.Put(stateRootTable, mainShardKey, mainShardEntry.Block.ChildBlocksRootHash.Bytes()); err != nil {
		return fmt.Errorf("failed to put state root: %w", err)
	}

	if err := bs.setParentOfNextToPropose(tx, mainShardEntry.Block.Hash); err != nil {
		return err
	}

	return bs.commit(tx)
}

func isValidProposalCandidate(entry *blockEntry, parentHash common.Hash) bool {
	return entry.Block.ShardId == types.MainShardId &&
		entry.IsProved &&
		entry.Block.ParentHash == parentHash
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

func (bs *BlockStorage) validateMainShardEntry(tx db.RoTx, id scTypes.BlockId, entry *blockEntry) error {
	if entry == nil {
		return fmt.Errorf("block with id=%s is not found", id.String())
	}

	if entry.Block.ShardId != types.MainShardId {
		return fmt.Errorf("block with id=%s is not from main shard", id.String())
	}

	if !entry.IsProved {
		return fmt.Errorf("block with id=%s is not proved", id.String())
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

// ResetProgressPartial resets the block storage state starting from the given main block hash:
//
//  1. Sets the latest fetched block reference to the parent of the block with hash == firstMainHashToPurge.
//     If the specified block is the first block in the chain, the new latest fetched value will be nil.
//
//  2. Deletes all main and corresponding exec shard blocks starting from the block with hash == firstMainHashToPurge.
func (bs *BlockStorage) ResetProgressPartial(ctx context.Context, firstMainHashToPurge common.Hash) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.resetProgressPartialImpl(ctx, firstMainHashToPurge)
	})
}

func (bs *BlockStorage) resetProgressPartialImpl(ctx context.Context, firstMainHashToPurge common.Hash) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	startingId := scTypes.NewBlockId(types.MainShardId, firstMainHashToPurge)

	startingEntry, err := bs.getBlockEntry(tx, startingId, true)
	if err != nil {
		return err
	}
	if err := bs.resetToParent(tx, startingEntry); err != nil {
		return err
	}

	for entry, err := range bs.getChainSequence(tx, startingId) {
		if err != nil {
			return err
		}

		if err := bs.deleteMainBlockWithChildren(tx, entry); err != nil {
			return err
		}
	}

	return bs.commit(tx)
}

func (bs *BlockStorage) resetToParent(tx db.RwTx, entry *blockEntry) error {
	refToParent, err := scTypes.GetMainParentRef(&entry.Block)
	if err != nil {
		return fmt.Errorf("failed to get main block parent ref: %w", err)
	}
	if err := bs.putLatestFetchedBlockTx(tx, types.MainShardId, refToParent); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}
	if err := bs.putLatestBatchIdTx(tx, entry.ParentBatchId); err != nil {
		return fmt.Errorf("failed to reset latest batch id: %w", err)
	}

	return nil
}

// getChainSequence iterates through a chain of blocks, starting from the block with the given id.
// It uses blockParentIdxTable to retrieve parent-child connections between blocks.
func (bs *BlockStorage) getChainSequence(tx db.RoTx, startingId scTypes.BlockId) iter.Seq2[*blockEntry, error] {
	return func(yield func(*blockEntry, error) bool) {
		startBlock, err := bs.getBlockEntry(tx, startingId, true)
		if err != nil {
			yield(nil, err)
			return
		}

		if !yield(startBlock, nil) {
			return
		}

		nextParentId := scTypes.IdFromBlock(&startBlock.Block)
		for {
			nextIdBytes, err := tx.Get(blockParentIdxTable, nextParentId.Bytes())
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				yield(nil, fmt.Errorf("failed to get parent idx entry, parentId=%s: %w", nextParentId, err))
				return
			}
			if nextIdBytes == nil {
				break
			}
			nextBlockEntry, err := bs.getBlockEntryBytesId(tx, nextIdBytes, true)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(nextBlockEntry, nil) {
				return
			}
			nextParentId = scTypes.IdFromBlock(&nextBlockEntry.Block)
		}
	}
}

// ResetProgressNotProved resets the block storage state:
//
//  1. Sets the latest fetched block reference to nil.
//
//  2. Deletes all main not yet proved blocks from the storage.
func (bs *BlockStorage) ResetProgressNotProved(ctx context.Context) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.resetProgressNotProvenImpl(ctx)
	})
}

func (bs *BlockStorage) resetProgressNotProvenImpl(ctx context.Context) error {
	tx, err := bs.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := bs.putLatestFetchedBlockTx(tx, types.MainShardId, nil); err != nil {
		return fmt.Errorf("failed to reset latest fetched block: %w", err)
	}

	for entry, err := range bs.storedBlocksIter(tx) {
		if err != nil {
			return err
		}
		if entry.Block.ShardId != types.MainShardId || entry.IsProved {
			continue
		}

		if err := bs.deleteMainBlockWithChildren(tx, entry); err != nil {
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

	entry, err := unmarshallEntry(idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (bs *BlockStorage) deleteMainBlockWithChildren(tx db.RwTx, mainShardEntry *blockEntry) error {
	childIds, err := scTypes.ChildBlockIds(&mainShardEntry.Block)
	if err != nil {
		return err
	}

	blocksCount := int32(len(childIds) + 1)
	if err := bs.addStoredCountTx(tx, -blocksCount); err != nil {
		return err
	}

	for _, childId := range childIds {
		childEntry, err := bs.getBlockEntry(tx, childId, true)
		if err != nil {
			return fmt.Errorf("failed to get child block with id=%s: %w", childId, err)
		}
		if err := bs.deleteBlock(tx, childEntry); err != nil {
			return err
		}
	}

	if err := bs.deleteBlock(tx, mainShardEntry); err != nil {
		return err
	}

	return nil
}

func (bs *BlockStorage) putBlockTx(tx db.RwTx, entry *blockEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return err
	}

	blockId := scTypes.IdFromBlock(&entry.Block)
	if err := tx.Put(blocksTable, blockId.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put block %s: %w", blockId.String(), err)
	}
	parentId := scTypes.ParentBlockId(&entry.Block)
	if err := tx.Put(blockParentIdxTable, parentId.Bytes(), blockId.Bytes()); err != nil {
		return fmt.Errorf("failed to put parent idx entry, parentId=%s: %w", parentId, err)
	}

	return nil
}

func (bs *BlockStorage) deleteBlock(tx db.RwTx, entry *blockEntry) error {
	parentBlockId := scTypes.ParentBlockId(&entry.Block)
	err := tx.Delete(blockParentIdxTable, parentBlockId.Bytes())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete parent idx entry, parentId=%s: %w", parentBlockId, err)
	}

	blockId := scTypes.IdFromBlock(&entry.Block)
	if err := tx.Delete(blocksTable, blockId.Bytes()); err != nil {
		return fmt.Errorf("failed to delete block with id=%s: %w", blockId, err)
	}

	return nil
}

func (*BlockStorage) storedBlocksIter(tx db.RoTx) iter.Seq2[*blockEntry, error] {
	return func(yield func(*blockEntry, error) bool) {
		txIter, err := tx.Range(blocksTable, nil, nil)
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
			entry, err := unmarshallEntry(key, val)
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
	currentBlocksCount, err := bs.getBlocksCountTx(tx)
	if err != nil {
		return err
	}

	signed := int32(currentBlocksCount) + delta
	if signed < 0 {
		return fmt.Errorf(
			"blocks count cannot be negative: delta=%d, current blocks count=%d", delta, currentBlocksCount,
		)
	}

	newBlocksCount := uint32(signed)
	if newBlocksCount > bs.config.CapacityLimit {
		return fmt.Errorf(
			"%w: delta is %d, current storage size is %d, capacity limit is %d",
			ErrCapacityLimitReached, delta, currentBlocksCount, bs.config.CapacityLimit,
		)
	}

	return bs.putBlocksCountTx(tx, newBlocksCount)
}

func (bs *BlockStorage) getBlocksCountTx(tx db.RoTx) (uint32, error) {
	bytes, err := tx.Get(storedBlocksCountTable, mainShardKey)
	switch {
	case err == nil:
		break
	case errors.Is(err, db.ErrKeyNotFound):
		return 0, nil
	default:
		return 0, fmt.Errorf("failed to get blocks count: %w", err)
	}

	count := binary.LittleEndian.Uint32(bytes)
	return count, nil
}

func (bs *BlockStorage) putBlocksCountTx(tx db.RwTx, newValue uint32) error {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, newValue)

	err := tx.Put(storedBlocksCountTable, mainShardKey, bytes)
	if err != nil {
		return fmt.Errorf("failed to put blocks count: %w (newValue is %d)", err, newValue)
	}
	return nil
}

func marshallEntry(entry *blockEntry) ([]byte, error) {
	bytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: failed to encode block with hash %s: %w", ErrSerializationFailed, entry.Block.Hash, err,
		)
	}
	return bytes, nil
}

func unmarshallEntry(key []byte, val []byte) (*blockEntry, error) {
	entry := &blockEntry{}
	if err := json.Unmarshal(val, entry); err != nil {
		return nil, fmt.Errorf(
			"%w: failed to unmarshall block entry with id=%s: %w", ErrSerializationFailed, hex.EncodeToString(key), err,
		)
	}

	return entry, nil
}
