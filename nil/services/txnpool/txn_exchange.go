package txnpool

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func topicPendingTransactions(shardId types.ShardId) string {
	return fmt.Sprintf("/shard/%s/pending-transactions", shardId)
}

func PublishPendingTransaction(
	ctx context.Context,
	networkManager network.Manager,
	shardId types.ShardId,
	txn *metaTxn,
) error {
	if networkManager == nil {
		// we don't always want to run the network (e.g., in tests)
		return nil
	}

	data, err := txn.MarshalSSZ()
	if err != nil {
		return fmt.Errorf("failed to marshal txn: %w", err)
	}

	return networkManager.PubSub().Publish(ctx, topicPendingTransactions(shardId), data)
}
