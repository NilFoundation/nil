package storage

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
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

func makeShardKey(shardId types.ShardId) []byte {
	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, uint32(shardId))
	return key
}
