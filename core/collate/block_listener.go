package collate

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/network"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

// StartBlockListeners starts listening to blocks on the given shards and saving them to the db.
// The call blocks the thread.
// It exits silently on context cancellation.
func StartBlockListeners(ctx context.Context, networkManager *network.Manager, dbAccessor db.DB, shardIds []types.ShardId) error {
	funcs := make([]concurrent.Func, 0, len(shardIds))
	for _, shardId := range shardIds {
		funcs = append(funcs, func(ctx context.Context) error {
			logger := logging.NewLogger("block_listener").With().
				Stringer(logging.FieldShardId, shardId).
				Logger()

			sub, err := networkManager.PubSub().Subscribe(topicShardBlocks(shardId))
			if err != nil {
				return err
			}
			defer sub.Close()

			ch, err := sub.Start(ctx)
			if err != nil {
				return err
			}

			for msg := range ch {
				if err := handleBlock(ctx, msg, shardId, dbAccessor, logger); err != nil {
					logger.Error().Err(err).Msg("Failed to handle block")
				}
			}

			return nil
		})
	}

	return concurrent.Run(ctx, funcs...)
}

func topicShardBlocks(shardId types.ShardId) string {
	return fmt.Sprintf("nil/shard/%s/blocks", shardId)
}

func handleBlock(ctx context.Context, msg []byte, shardId types.ShardId, dbAccessor db.DB, logger zerolog.Logger) error {
	block := &types.Block{}
	if err := block.UnmarshalSSZ(msg); err != nil {
		return fmt.Errorf("failed to unmarshal block: %w", err)
	}

	logger.Debug().
		Stringer(logging.FieldBlockNumber, block.Id).
		Stringer(logging.FieldBlockHash, block.Hash()).
		Msg("Received block")

	tx, err := dbAccessor.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// todo: write to db
	_ = shardId
	// if err := db.WriteBlock(tx, shardId, block); err != nil {
	//	return err
	// }
	// return tx.Commit()

	return nil
}

// PublishBlock publishes a block to the network.
func PublishBlock(ctx context.Context, networkManager *network.Manager, shardId types.ShardId, block *types.Block) error {
	if networkManager == nil {
		// todo: this is for the start when we don't always want to run the network
		return nil
	}

	data, err := block.MarshalSSZ()
	if err != nil {
		return err
	}

	return networkManager.PubSub().Publish(ctx, topicShardBlocks(shardId), data)
}
