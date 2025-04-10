package storage

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

var mainShardKey = marshallShardId(types.MainShardId)

type batchEntry struct {
	Id       scTypes.BatchId     `json:"batchId"`
	ParentId *scTypes.BatchId    `json:"parentBatchId,omitempty"`
	Status   scTypes.BatchStatus `json:"status"`

	ParentRefs          map[types.ShardId]*scTypes.BlockRef `json:"parentRefs"`
	LatestMainBlockHash common.Hash                         `json:"latestMainBlockHash"`
	BlockIds            []scTypes.BlockId                   `json:"blockIds"`
	DataProofs          scTypes.DataProofs                  `json:"dataProofs"`

	IsProved  bool      `json:"isProved,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func newBatchEntry(batch *scTypes.BlockBatch, createdAt time.Time) *batchEntry {
	parentRefs := batch.ParentRefs()
	mainRef, ok := parentRefs[types.MainShardId]
	check.PanicIfNotf(ok && mainRef != nil, "batch must have a parent ref for main shard, id=%s", batch.Id)

	return &batchEntry{
		Id:       batch.Id,
		ParentId: batch.ParentId,
		Status:   batch.Status,

		ParentRefs:          batch.ParentRefs(),
		LatestMainBlockHash: batch.LatestMainBlock().Hash,
		BlockIds:            batch.BlockIds(),
		DataProofs:          batch.DataProofs,

		IsProved:  false,
		CreatedAt: createdAt,
	}
}

func (e *batchEntry) IsValidProposalCandidate(currentStateRoot common.Hash) bool {
	mainParentRef, ok := e.ParentRefs[types.MainShardId]
	if !ok {
		return false
	}

	return e.IsProved && mainParentRef.Hash == currentStateRoot
}

type blockEntry struct {
	Block     scTypes.Block   `json:"block"`
	BatchId   scTypes.BatchId `json:"batchId"`
	FetchedAt time.Time       `json:"fetchedAt"`
}

func newBlockEntry(block *scTypes.Block, containingBatch *scTypes.BlockBatch, fetchedAt time.Time) *blockEntry {
	return &blockEntry{
		Block:     *block,
		BatchId:   containingBatch.Id,
		FetchedAt: fetchedAt,
	}
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

func marshallShardId(shardId types.ShardId) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(shardId))
	return key
}

func unmarshallShardId(key []byte) types.ShardId {
	return types.ShardId(binary.LittleEndian.Uint32(key))
}

func tableIter[Entry any](tx db.RoTx, table db.TableName) iter.Seq2[*Entry, error] {
	return func(yield func(*Entry, error) bool) {
		txIter, err := tx.Range(table, nil, nil)
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
			entry, err := unmarshallEntry[Entry](key, val)
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
