package collate

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const topicVersion = "/nil/version"

func topicBootstrapShard() network.ProtocolID {
	return "/nil/snap"
}

func createShardBootstrapHandler(ctx context.Context, database db.DB, logger logging.Logger) func(s network.Stream) {
	dummyFilter := func([]byte) bool { return true }
	return func(s network.Stream) {
		defer s.Close()

		logger.Info().Msg("New peer for snapshot downloading connected")

		if err := s.CloseRead(); err != nil {
			logger.Error().Err(err).Msg("Failed to close read stream")
			return
		}

		if err := database.Stream(ctx, dummyFilter, s); err != nil {
			logger.Error().Err(err).Msg("Stream error")
		}

		logger.Info().Msg("Snapshot downloaded")
	}
}

// SetBootstrapHandler sets a handler that streams DB data via libp2p.
func SetBootstrapHandler(ctx context.Context, nm network.Manager, db db.DB) {
	logger := logging.NewLogger("bootstrap").With().
		Stringer(logging.FieldP2PIdentity, nm.ID()).
		Logger()

	nm.SetStreamHandler(
		ctx,
		topicBootstrapShard(),
		createShardBootstrapHandler(ctx, db, logger),
	)

	logger.Info().Msg("Enabled bootstrap endpoint")
}

func SetVersionHandler(ctx context.Context, nm network.Manager, fabric db.DB) error {
	tx, err := fabric.CreateRoTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	// The genesis block must have been initialized before this method is called.
	version, err := db.ReadBlockHashByNumber(tx, types.MainShardId, 0)
	if err != nil {
		return fmt.Errorf("failed to read genesis block hash: %w", err)
	}
	check.PanicIfNot(!version.Empty())

	resp, err := version.MarshalSSZ()
	if err != nil {
		return fmt.Errorf("failed to marshal genesis block hash: %w", err)
	}

	nm.SetRequestHandler(ctx, topicVersion, func(ctx context.Context, _ []byte) ([]byte, error) {
		return resp, nil
	})

	return nil
}

func fetchShardSnap(
	ctx context.Context,
	nm network.Manager,
	peerId network.PeerID,
	db db.DB,
	logger logging.Logger,
) error {
	stream, err := nm.NewStream(ctx, peerId, topicBootstrapShard())
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to open stream to %s", peerId)
		return err
	}
	defer stream.Close()

	logger.Info().Msgf("Start to fetch data snapshot from %s", peerId)
	// TODO: Here we need to check signatures of fetched blocks,
	//  this would require checking every block in the stream, which can be slow.
	if err := db.Fetch(ctx, stream); err != nil {
		logger.Error().Err(err).Msgf("Failed to fetch snapshot from %s", peerId)
		return err
	}
	logger.Info().Msg("Fetching snapshot completed")
	return nil
}

// Fetch DB snapshot via libp2p.
func fetchSnapshot(
	ctx context.Context,
	nm network.Manager,
	peerAddr network.AddrInfo,
	db db.DB,
	logger logging.Logger,
) error {
	peerId, err := nm.Connect(ctx, peerAddr)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to connect to %s to fetch snapshot", peerAddr)
		return err
	}
	return fetchShardSnap(ctx, nm, peerId, db, logger)
}

func fetchGenesisBlockHash(ctx context.Context, nm network.Manager, peerId network.PeerID) (common.Hash, error) {
	resp, err := nm.SendRequestAndGetResponse(ctx, peerId, topicVersion, nil)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to fetch genesis block hash: %w", err)
	}

	var res common.Hash
	if err := res.UnmarshalSSZ(resp); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to unmarshal genesis block hash: %w", err)
	}

	return res, nil
}
