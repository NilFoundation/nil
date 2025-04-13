package collate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	cerrors "github.com/NilFoundation/nil/nil/internal/collate/errors"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	cm "github.com/NilFoundation/nil/nil/internal/network/connection_manager"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/multiformats/go-multistream"
	"google.golang.org/protobuf/proto"
)

type ProtocolVersionMismatchError struct {
	LocalVersion  string
	RemoteVersion string
}

func (e *ProtocolVersionMismatchError) Error() string {
	return fmt.Sprintf("protocol version mismatch; local: %s, remote: %s", e.LocalVersion, e.RemoteVersion)
}

type SyncerConfig struct {
	execution.BlockGeneratorParams

	Name            string
	ShardId         types.ShardId
	Timeout         time.Duration // pull blocks if no new blocks appear in the topic for this duration
	BootstrapPeers  []network.AddrInfo
	ZeroStateConfig *execution.ZeroStateConfig
}

// every n-th block will be reported to info log (to avoid spamming)
const blockReportInterval = 100

type Syncer struct {
	config *SyncerConfig

	topic string

	db             db.DB
	networkManager network.Manager

	logger logging.Logger

	waitForSync *sync.WaitGroup

	validator *Validator
}

func NewSyncer(cfg *SyncerConfig, validator *Validator, db db.DB, networkManager network.Manager) (*Syncer, error) {
	var waitForSync sync.WaitGroup
	waitForSync.Add(1)

	return &Syncer{
		config:         cfg,
		topic:          topicShardBlocks(cfg.ShardId),
		db:             db,
		networkManager: networkManager,
		logger: logging.NewLogger(cfg.Name).With().
			Stringer(logging.FieldShardId, cfg.ShardId).
			Logger(),
		waitForSync: &waitForSync,
		validator:   validator,
	}, nil
}

func (s *Syncer) shardIsEmpty(ctx context.Context) (bool, error) {
	block, _, err := s.validator.GetLastBlock(ctx)
	if err != nil {
		return false, err
	}
	return block == nil, nil
}

func (s *Syncer) WaitComplete(ctx context.Context) error {
	c := make(chan struct{}, 1)
	go func() {
		defer close(c)
		s.waitForSync.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c:
		return nil
	}
}

func (s *Syncer) getLocalVersion(ctx context.Context) (*NodeVersion, error) {
	protocolVersion := s.networkManager.ProtocolVersion()

	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer rotx.Rollback()

	res, err := db.ReadBlockHashByNumber(rotx, types.MainShardId, 0)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return &NodeVersion{protocolVersion, common.EmptyHash}, nil
		}
		return nil, err
	}
	return &NodeVersion{protocolVersion, res}, err
}

type NodeVersion struct {
	ProtocolVersion  string
	GenesisBlockHash common.Hash
}

func (s *Syncer) fetchRemoteVersion(ctx context.Context) (NodeVersion, error) {
	var err error
	for _, peer := range s.config.BootstrapPeers {
		var peerId network.PeerID
		peerId, err = s.networkManager.Connect(ctx, peer)
		if err != nil {
			continue
		}

		var protocolVersion string
		protocolVersion, err = s.networkManager.GetPeerProtocolVersion(peerId)
		if err != nil {
			continue
		}

		var res common.Hash
		res, err = fetchGenesisBlockHash(ctx, s.networkManager, peerId)
		if err == nil {
			return NodeVersion{protocolVersion, res}, nil
		}
	}
	return NodeVersion{}, fmt.Errorf("failed to fetch version from all peers; last error: %w", err)
}

func (s *Syncer) fetchSnapshot(ctx context.Context) error {
	if len(s.config.BootstrapPeers) == 0 {
		s.logger.Warn().Msg("No bootstrap peers to fetch snapshot from")
		return nil
	}

	var err error
	for _, peer := range s.config.BootstrapPeers {
		err = fetchSnapshot(ctx, s.networkManager, peer, s.db, s.logger)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed to fetch snapshot from all peers; last error: %w", err)
}

func (s *Syncer) Init(ctx context.Context, allowDbDrop bool) error {
	if s.networkManager == nil {
		return nil
	}

	version, err := s.getLocalVersion(ctx)
	if err != nil {
		return err
	}

	remoteVersion, err := s.fetchRemoteVersion(ctx)
	if err != nil {
		// todo: when all shards can handle the new protocol, we should return an error here
		s.logger.Warn().Err(err).Msgf(
			"Failed to fetch remote version. For now we assume that local version %s is up to date", version)
		return nil
	}

	if version.ProtocolVersion != remoteVersion.ProtocolVersion {
		return &ProtocolVersionMismatchError{
			version.ProtocolVersion,
			remoteVersion.ProtocolVersion,
		}
	}

	if version.GenesisBlockHash.Empty() {
		s.logger.Info().Msg("Local version is empty. Fetching snapshot...")
		return s.fetchSnapshot(ctx)
	}

	if version.GenesisBlockHash == remoteVersion.GenesisBlockHash {
		s.logger.Info().Msgf("Local version %s is up to date. Finished initialization", version)
		return nil
	}

	if !allowDbDrop {
		return fmt.Errorf("local version is outdated; local: %s, remote: %s", version, remoteVersion)
	}

	s.logger.Info().Msg("Local version is outdated. Dropping db...")
	if err := s.db.DropAll(); err != nil {
		return fmt.Errorf("failed to drop db: %w", err)
	}
	s.logger.Info().Msg("DB dropped. Fetching snapshot...")
	return s.fetchSnapshot(ctx)
}

func (s *Syncer) SetHandlers(ctx context.Context) error {
	if s.networkManager == nil {
		return nil
	}

	if err := SetVersionHandler(ctx, s.networkManager, s.db); err != nil {
		return fmt.Errorf("failed to set version handler: %w", err)
	}

	SetBootstrapHandler(ctx, s.networkManager, s.db)
	return nil
}

func (s *Syncer) Run(ctx context.Context) error {
	if s.networkManager == nil {
		s.waitForSync.Done()
		return nil
	}

	s.logger.Info().Msg("Starting sync...")

	s.fetchBlocks(ctx)
	s.waitForSync.Done()

	if ctx.Err() != nil {
		return nil
	}

	sub, err := s.networkManager.PubSub().Subscribe(s.topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", s.topic, err)
	}
	defer sub.Close()

	ch := sub.Start(ctx, true)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug().Msg("Syncer is terminated")
			return nil
		case msg := <-ch:
			saved, err := s.processTopicTransaction(ctx, msg.Data)
			if err != nil {
				if errors.As(err, new(invalidSignatureError)) {
					peerReputationTracker := network.TryGetPeerReputationTracker(s.networkManager)
					if peerReputationTracker != nil {
						peerReputationTracker.ReportPeer(msg.ReceivedFrom, cm.ReputationChangeInvalidBlockSignature)
					} else {
						s.logger.Warn().Msg("Peer reputation tracker is not available")
					}
				}
				s.logger.Error().Err(err).Msg("Failed to process topic transaction")
			}
			if !saved {
				s.fetchBlocks(ctx)
			}
		case <-time.After(s.config.Timeout):
			s.logger.Warn().Msgf("No new block in the topic for %s, pulling blocks actively", s.config.Timeout)

			s.fetchBlocks(ctx)
		}
	}
}

func (s *Syncer) processTopicTransaction(ctx context.Context, data []byte) (bool, error) {
	var pbBlock pb.RawFullBlock
	if err := proto.Unmarshal(data, &pbBlock); err != nil {
		return false, err
	}
	b, err := unmarshalBlockSSZ(&pbBlock)
	if err != nil {
		return false, err
	}

	block := b.Block
	s.logger.Debug().
		Stringer(logging.FieldBlockNumber, block.Id).
		Msg("Received block")

	if err := s.saveBlock(ctx, b); err != nil {
		switch {
		case errors.Is(err, cerrors.ErrOutOfOrder):
			// todo: queue the block for later processing
			return false, nil
		case errors.Is(err, cerrors.ErrOldBlock):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (s *Syncer) fetchBlocks(ctx context.Context) {
	// todo: fetch blocks until the queue (see todo above) is empty
	for {
		s.logger.Trace().Msg("Fetching next blocks")

		blocksCh := s.fetchBlocksRange(ctx)
		if blocksCh == nil {
			return
		}
		var count int
		for block := range blocksCh {
			count++
			if err := s.saveBlock(ctx, block); err != nil {
				if errors.Is(err, cerrors.ErrOldBlock) {
					continue
				}
				s.logger.Error().
					Err(err).
					Stringer(logging.FieldBlockNumber, block.Id).
					Msg("Failed to save block")
				return
			}
		}
		if count == 0 {
			s.logger.Trace().Msg("No new blocks to fetch")
			return
		}
	}
}

func (s *Syncer) fetchBlocksRange(ctx context.Context) <-chan *types.BlockWithExtractedData {
	peers := ListPeers(s.networkManager, s.config.ShardId)

	if len(peers) == 0 {
		s.logger.Warn().Msg("No peers to fetch block from")
		return nil
	}

	s.logger.Trace().Msgf("Found %d peers to fetch block from:\n%v", len(peers), peers)

	lastBlock, _, err := s.validator.GetLastBlock(ctx)
	if err != nil {
		return nil
	}
	check.PanicIfNotf(
		lastBlock != nil,
		"No last block found. If the syncers were correctly initialized, this should be impossible.")

	for _, p := range peers {
		s.logger.Trace().Msgf("Requesting blocks from %d from peer %s", lastBlock.Id+1, p)

		blocksCh, err := RequestBlocks(ctx, s.networkManager, p, s.config.ShardId, lastBlock.Id+1, s.logger)
		if err == nil {
			return blocksCh
		}

		if errors.As(err, &multistream.ErrNotSupported[network.ProtocolID]{}) {
			s.logger.Debug().Err(err).Msgf("Peer %s does not support the block protocol with our shard", p)
		} else {
			s.logger.Warn().Err(err).Msgf("Failed to request block from peer %s", p)
		}
	}

	return nil
}

func (s *Syncer) saveBlock(ctx context.Context, block *types.BlockWithExtractedData) error {
	if err := s.validator.ReplayBlock(ctx, block); err != nil {
		return err
	}

	if uint64(block.Id)%uint64(blockReportInterval) == 0 {
		s.logger.Info().
			Uint64(logging.FieldBlockNumber, uint64(block.Id)).
			Stringer(logging.FieldBlockHash, block.Hash(s.config.ShardId)).
			Msg("Saved block")
	}

	s.logger.Trace().
		Stringer(logging.FieldBlockNumber, block.Id).
		Msg("Block written")

	return nil
}

func (s *Syncer) GenerateZerostateIfShardIsEmpty(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	if empty, err := s.shardIsEmpty(ctx); err != nil {
		return err
	} else if !empty {
		return nil
	}

	s.logger.Info().Msg("Generating zero-state...")
	gen, err := execution.NewBlockGenerator(ctx, s.config.BlockGeneratorParams, s.db, nil)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	if _, err := gen.GenerateZeroState(s.config.ZeroStateConfig); err != nil {
		return err
	}
	return nil
}

func returnErrorOrPanic(err error) error {
	if assert.Enable {
		check.PanicIfErr(err)
	}
	return err
}
