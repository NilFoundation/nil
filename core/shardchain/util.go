package shardchain

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

func GetLastBlockHash(roTx db.Tx, shardId types.ShardId) (common.Hash, error) {
	lastBlockRaw, err := roTx.Get(db.LastBlockTable, shardId.Bytes())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, fmt.Errorf("failed getting last block %w for shard %d", err, shardId)
	}
	lastBlockHash := common.EmptyHash
	if lastBlockRaw != nil {
		lastBlockHash = common.Hash(*lastBlockRaw)
	}
	return lastBlockHash, nil
}
