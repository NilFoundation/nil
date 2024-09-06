package collate

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

func topicBootstrapShard(shardId types.ShardId) network.ProtocolID {
	return network.ProtocolID(fmt.Sprintf("nil/shard/%s/snap", shardId))
}

func createShardBootstrapHandler(ctx context.Context, shardId types.ShardId, database db.DB, logger zerolog.Logger) func(s network.Stream) {
	filter := db.CreateKeyFromShardTableChecker(shardId)

	return func(s network.Stream) {
		defer s.Close()

		logger := logger.With().Str(logging.FieldP2PIdentity, s.ID()).Logger()
		logger.Info().Msg("New peer for snapshot downloading connected")

		if err := s.CloseRead(); err != nil {
			logger.Error().Err(err).Msg("Failed to close read stream")
			return
		}

		if err := database.Stream(ctx, filter, s); err != nil {
			logger.Error().Err(err).Msg("Stream error")
		}

		logger.Info().Msg("Snapshot downloaded")
	}
}

// Set handler that streams DB data via libp2p.
func SetBootstrapHandler(ctx context.Context, nm *network.Manager, shardId types.ShardId, db db.DB) error {
	if nm == nil {
		return nil
	}

	logger := logging.NewLogger("bootstrap").With().Stringer(logging.FieldShardId, shardId).Logger()

	nm.SetStreamHandler(
		topicBootstrapShard(shardId),
		createShardBootstrapHandler(ctx, shardId, db, logger),
	)

	logger.Info().Msg("Enable bootstrap endpoint")

	return nil
}

func fetchShardSnap(ctx context.Context, nm *network.Manager, peerId network.PeerID, shardId types.ShardId, db db.DB, logger zerolog.Logger) error {
	stream, err := nm.NewStream(ctx, peerId, topicBootstrapShard(shardId))
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to open stream to %s", peerId)
		return err
	}

	if err := db.Fetch(ctx, stream); err != nil {
		logger.Error().Err(err).Msgf("Failed to fetch snapshot from %s", peerId)
		return err
	}
	logger.Info().Msgf("Fetching snapshot for shard %s is completed", shardId)
	return nil
}

// Fetch DB snapshot via libp2p.
func FetchSnapshot(ctx context.Context, nm *network.Manager, peerAddr string, shardId types.ShardId, db db.DB) error {
	if nm == nil {
		return nil
	}

	logger := logging.NewLogger("bootstrap").With().Stringer(logging.FieldShardId, shardId).Logger()
	if peerAddr == "" {
		logger.Info().Msg("Peer address is empty. Snapshot won't be fetched")
		return nil
	}

	logger.Info().Msgf("Start to fetch data snapshot from %s", peerAddr)

	peerId, err := nm.Connect(ctx, peerAddr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to connect to %s", peerAddr)
		return err
	}
	return fetchShardSnap(ctx, nm, peerId, shardId, db, logger)
}
