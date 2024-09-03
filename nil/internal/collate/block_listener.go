package collate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Block struct {
	Block       *types.Block     `json:"block"`
	OutMessages []*types.Message `json:"outMessages,omitempty"`
}

func topicShardBlocks(shardId types.ShardId) string {
	return fmt.Sprintf("nil/shard/%s/blocks", shardId)
}

// PublishBlock publishes a block to the network.
func PublishBlock(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, block *Block) error {
	if networkManager == nil {
		// todo: this is for the start when we don't always want to run the network
		return nil
	}

	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	return networkManager.PubSub().Publish(ctx, topicShardBlocks(shardId), data)
}
