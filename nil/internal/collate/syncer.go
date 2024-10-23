package collate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate/pb"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/multiformats/go-multistream"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

type SyncerConfig struct {
	ShardId       types.ShardId
	Timeout       time.Duration // pull blocks if no new blocks appear in the topic for this duration
	BootstrapPeer string
	ReplayBlocks  bool // replay blocks (archive node) or store headers and messages only

	BlockGeneratorParams execution.BlockGeneratorParams
}

type Syncer struct {
	config SyncerConfig

	topic string

	db             db.DB
	networkManager *network.Manager

	logger zerolog.Logger

	lastBlockNumber types.BlockNumber
	lastBlockHash   common.Hash
}

func NewSyncer(cfg SyncerConfig, db db.DB, networkManager *network.Manager) *Syncer {
	return &Syncer{
		config:         cfg,
		topic:          topicShardBlocks(cfg.ShardId),
		db:             db,
		networkManager: networkManager,
		logger: logging.NewLogger("sync").With().
			Stringer(logging.FieldShardId, cfg.ShardId).
			Logger(),
		lastBlockNumber: types.BlockNumber(math.MaxUint64),
	}
}

func (s *Syncer) readLastBlockNumber(ctx context.Context) error {
	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer rotx.Rollback()
	lastBlock, lastBlockHash, err := db.ReadLastBlock(rotx, s.config.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if err == nil {
		s.lastBlockNumber = lastBlock.Id
		s.lastBlockHash = lastBlockHash
	}
	return nil
}

func (s *Syncer) snapIsRequired(ctx context.Context) (bool, error) {
	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer rotx.Rollback()

	_, _, err = db.ReadLastBlock(rotx, s.config.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return false, err
	}
	return err != nil, nil
}

func (s *Syncer) Run(ctx context.Context, wgFetch *sync.WaitGroup) error {
	if required, err := s.snapIsRequired(ctx); err != nil {
		return err
	} else if required {
		if err := FetchSnapshot(ctx, s.networkManager, s.config.BootstrapPeer, s.config.ShardId, s.db); err != nil {
			return fmt.Errorf("failed to fetch snapshot: %w", err)
		}
	}

	// Wait until snapshots for shards are fetched.
	// It's impossible to load data and commit transactions at the same time.
	wgFetch.Done()
	wgFetch.Wait()

	err := s.readLastBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to read last block number: %w", err)
	}

	s.logger.Debug().
		Stringer(logging.FieldBlockHash, s.lastBlockHash).
		Uint64(logging.FieldBlockNumber, s.lastBlockNumber.Uint64()).
		Msgf("Initialized sync collator at starting block")

	s.logger.Info().Msg("Starting sync")

	sub, err := s.networkManager.PubSub().Subscribe(s.topic)
	if err != nil {
		return err
	}
	defer sub.Close()

	ch := sub.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug().Msg("Sync collator is terminated")
			return nil
		case data := <-ch:
			saved, err := s.processTopicMessage(ctx, data)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to process topic message")
			}
			if !saved {
				s.fetchBlocks(ctx)
			}
		case <-time.After(s.config.Timeout):
			s.logger.Debug().Msgf("No new block in the topic for %s, pulling blocks actively", s.config.Timeout)

			s.fetchBlocks(ctx)
		}
	}
}

func (s *Syncer) processTopicMessage(ctx context.Context, data []byte) (bool, error) {
	var pbBlock pb.Block
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
		Stringer(logging.FieldBlockHash, block.Hash()).
		Msg("Received block")

	if block.Id != s.lastBlockNumber+1 {
		s.logger.Debug().
			Stringer(logging.FieldBlockNumber, block.Id).
			Msgf("Received block is out of order with the last block %d", s.lastBlockNumber)

		// todo: queue the block for later processing
		return false, nil
	}

	if block.PrevBlock != s.lastBlockHash {
		msg := fmt.Sprintf("Prev block hash mismatch: expected %x, got %x", s.lastBlockHash, block.PrevBlock)
		s.logger.Error().
			Stringer(logging.FieldBlockNumber, block.Id).
			Stringer(logging.FieldBlockHash, block.Hash()).
			Msg(msg)
		panic(msg)
	}

	if err := s.saveBlocks(ctx, []*Block{b}); err != nil {
		return false, err
	}

	return true, nil
}

func (s *Syncer) fetchBlocks(ctx context.Context) {
	// todo: fetch blocks until the queue (see todo above) is empty
	for {
		s.logger.Debug().Msg("Fetching next block")

		blocks := s.fetchBlocksRange(ctx)
		if len(blocks) == 0 {
			s.logger.Debug().Msg("No new blocks to fetch")
			return
		}
		if err := s.saveBlocks(ctx, blocks); err != nil {
			s.logger.Error().Err(err).Msg("Failed to save blocks")
			return
		}
	}
}

func (s *Syncer) fetchBlocksRange(ctx context.Context) []*Block {
	peers := ListPeers(s.networkManager, s.config.ShardId)

	if len(peers) == 0 {
		s.logger.Warn().Msg("No peers to fetch block from")
		return nil
	}

	s.logger.Debug().Msgf("Found %d peers to fetch block from:\n%v", len(peers), peers)

	for _, p := range peers {
		s.logger.Debug().Msgf("Requesting block %d from peer %s", s.lastBlockNumber+1, p)

		const count = 100
		blocks, err := RequestBlocks(ctx, s.networkManager, p, s.config.ShardId, s.lastBlockNumber+1, count)
		if err == nil {
			return blocks
		}

		if errors.As(err, &multistream.ErrNotSupported[network.ProtocolID]{}) {
			s.logger.Debug().Err(err).Msgf("Peer %s does not support the block protocol with our shard", p)
		} else {
			s.logger.Warn().Err(err).Msgf("Failed to request block from peer %s", p)
		}
	}

	return nil
}

func (s *Syncer) saveBlocks(ctx context.Context, blocks []*Block) error {
	if len(blocks) == 0 {
		return nil
	}

	if s.config.ReplayBlocks {
		if err := s.replayBlocks(ctx, blocks); err != nil {
			return err
		}
	} else {
		if err := s.saveDirectly(ctx, blocks); err != nil {
			return err
		}
	}

	s.lastBlockNumber = blocks[len(blocks)-1].Block.Id
	s.lastBlockHash = blocks[len(blocks)-1].Block.Hash()

	s.logger.Debug().
		Stringer(logging.FieldBlockNumber, s.lastBlockNumber).
		Msg("Blocks written")

	return nil
}

func (s *Syncer) saveDirectly(ctx context.Context, blocks []*Block) error {
	tx, err := s.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, block := range blocks {
		blockHash := block.Block.Hash()
		if err := db.WriteBlock(tx, s.config.ShardId, blockHash, block.Block); err != nil {
			return err
		}

		msgRoot, err := s.saveMessages(tx, block.OutMessages)
		if err != nil {
			return err
		}

		if msgRoot != block.Block.OutMessagesRoot {
			return errors.New("out messages root mismatch")
		}

		_, err = execution.PostprocessBlock(tx, s.config.ShardId, block.Block.GasPrice, blockHash)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Syncer) replayBlocks(ctx context.Context, blocks []*Block) error {
	for _, block := range blocks {
		gen, err := execution.NewBlockGenerator(ctx, s.config.BlockGeneratorParams, s.db)
		if err != nil {
			return err
		}

		blockHash := block.Block.Hash()
		s.logger.Debug().
			Stringer(logging.FieldBlockNumber, block.Block.Id).
			Stringer(logging.FieldBlockHash, blockHash).
			Msg("Replaying block")

		b, msgs, err := gen.GenerateBlock(&execution.Proposal{
			PrevBlockId:   block.Block.Id - 1,
			PrevBlockHash: block.Block.PrevBlock,
			MainChainHash: block.Block.MainChainHash,
			ShardHashes:   block.ShardHashes,
			InMsgs:        block.InMessages,
			ForwardMsgs: slices.DeleteFunc(slices.Clone(block.OutMessages),
				func(m *types.Message) bool { return m.From.ShardId() == s.config.ShardId }),
		})
		if err != nil {
			return err
		}

		if b.Hash() != blockHash {
			return errors.New("block hash mismatch")
		}
		if len(msgs) != len(block.OutMessages) {
			return errors.New("out messages count mismatch")
		}
	}

	return nil
}

func (s *Syncer) saveMessages(tx db.RwTx, messages []*types.Message) (common.Hash, error) {
	messageTree := execution.NewDbMessageTrie(tx, s.config.ShardId)
	indexes := make([]types.MessageIndex, len(messages))
	for i := range messages {
		indexes[i] = types.MessageIndex(i)
	}
	if err := messageTree.UpdateBatch(indexes, messages); err != nil {
		return common.EmptyHash, err
	}
	return messageTree.RootHash(), nil
}
