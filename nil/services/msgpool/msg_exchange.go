package msgpool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func topicPendingMessages(shardId types.ShardId) string {
	return fmt.Sprintf("nil/shard/%s/pending-messages", shardId)
}

func PublishPendingMessage(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, msg *metaMsg) error {
	if networkManager == nil {
		// we don't always want to run the network (e.g., in tests)
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal msg: %w", err)
	}

	return networkManager.PubSub().Publish(ctx, topicPendingMessages(shardId), data)
}
