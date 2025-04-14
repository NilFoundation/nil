package storage

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

var mainShardKey = marshallShardId(types.MainShardId)

type batchEntry struct {
	Id       scTypes.BatchId  `json:"batchId"`
	ParentId *scTypes.BatchId `json:"parentBatchId,omitempty"`

	LatestRefs scTypes.BlockRefs                   `json:"latestRefs,omitempty"`
	ParentRefs map[types.ShardId]*scTypes.BlockRef `json:"parentRefs,omitempty"`
	BlockIds   []scTypes.BlockId                   `json:"blockIds"`
	DataProofs scTypes.DataProofs                  `json:"dataProofs"`

	IsSealed  bool      `json:"isSealed,omitempty"`
	IsProved  bool      `json:"isProved,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newBatchEntry(batch *scTypes.BlockBatch) *batchEntry {
	return &batchEntry{
		Id:       batch.Id,
		ParentId: batch.ParentId,

		LatestRefs: batch.LatestRefs(),
		ParentRefs: batch.ParentRefs(),
		BlockIds:   batch.BlockIds(),
		DataProofs: batch.DataProofs,

		IsSealed:  batch.IsSealed,
		IsProved:  false,
		CreatedAt: batch.CreatedAt,
		UpdatedAt: batch.UpdatedAt,
	}
}

func (e *batchEntry) IsValidProposalCandidate(currentStateRoot common.Hash) bool {
	mainParentRef, ok := e.ParentRefs[types.MainShardId]
	if !ok {
		return false
	}

	if mainRef := e.LatestRefs.TryGetMain(); mainRef == nil {
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
