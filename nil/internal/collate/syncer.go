package collate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/signer"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/multiformats/go-multistream"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

type SyncerConfig struct {
	ShardId       types.ShardId
	Timeout       time.Duration // pull blocks if no new blocks appear in the topic for this duration
	BootstrapPeer *network.AddrInfo
	ReplayBlocks  bool // replay blocks (archive node) or store headers and transactions only

	BlockGeneratorParams execution.BlockGeneratorParams
	ZeroState            string
	ZeroStateConfig      *execution.ZeroStateConfig
}

type Syncer struct {
	config SyncerConfig

	topic string

	db             db.DB
	networkManager *network.Manager

	logger zerolog.Logger

	waitForSync *sync.WaitGroup

	subsMutex     sync.Mutex
	subsId        uint64
	subs          map[uint64]chan struct{}
	blockVerifier *signer.BlockVerifier
}

func NewSyncer(cfg SyncerConfig, db db.DB, networkManager *network.Manager) (*Syncer, error) {
	var waitForSync sync.WaitGroup
	waitForSync.Add(1)

	return &Syncer{
		config:         cfg,
		topic:          topicShardBlocks(cfg.ShardId),
		db:             db,
		networkManager: networkManager,
		logger: logging.NewLogger("sync").With().
			Stringer(logging.FieldShardId, cfg.ShardId).
			Logger(),
		waitForSync:   &waitForSync,
		subs:          make(map[uint64]chan struct{}),
		blockVerifier: signer.NewBlockVerifier(cfg.ShardId, db),
	}, nil
}

func (s *Syncer) readLastBlock(ctx context.Context) (*types.Block, common.Hash, error) {
	rotx, err := s.db.CreateRoTx(ctx)
	if err != nil {
		return nil, common.EmptyHash, err
	}
	defer rotx.Rollback()

	block, hash, err := db.ReadLastBlock(rotx, s.config.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, common.EmptyHash, err
	}
	if err == nil {
		return block, hash, err
	}
	return nil, common.EmptyHash, nil
}

func (s *Syncer) shardIsEmpty(ctx context.Context) (bool, error) {
	block, _, err := s.readLastBlock(ctx)
	if err != nil {
		return false, err
	}
	return block == nil, nil
}

func (s *Syncer) WaitComplete() {
	s.waitForSync.Wait()
}

func (s *Syncer) Subscribe() (uint64, <-chan struct{}) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	ch := make(chan struct{}, 1)
	id := s.subsId
	s.subs[id] = ch
	s.subsId++
	return id, ch
}

func (s *Syncer) Unsubscribe(id uint64) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	close(s.subs[id])
	delete(s.subs, id)
}

func (s *Syncer) notify() {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	for _, ch := range s.subs {
		ch <- struct{}{}
	}
}

func (s *Syncer) FetchSnapshot(ctx context.Context) error {
	if s.config.ReplayBlocks {
		if snapIsRequired, err := s.shardIsEmpty(ctx); err != nil {
			return err
		} else if snapIsRequired {
			if err := FetchSnapshot(ctx, s.networkManager, s.config.BootstrapPeer, s.config.ShardId, s.db); err != nil {
				return fmt.Errorf("failed to fetch snapshot: %w", err)
			}
		}
	}

	s.db.FetcherDone()
	return nil
}

func (s *Syncer) Run(ctx context.Context) error {
	if s.networkManager == nil {
		s.waitForSync.Done()
		return nil
	}

	block, hash, err := s.readLastBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to read last block number: %w", err)
	}

	s.logger.Debug().
		Stringer(logging.FieldBlockHash, hash).
		Uint64(logging.FieldBlockNumber, uint64(block.Id)).
		Msg("Initialized sync proposer at starting block")

	s.logger.Info().Msg("Starting sync")

	s.fetchBlocks(ctx)
	s.waitForSync.Done()

	if ctx.Err() != nil {
		return nil
	}

	sub, err := s.networkManager.PubSub().Subscribe(s.topic)
	if err != nil {
		return fmt.Errorf("Failed to subscribe to %s: %w", s.topic, err)
	}
	defer sub.Close()

	ch := sub.Start(ctx, true)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug().Msg("Sync proposer is terminated")
			return nil
		case data := <-ch:
			saved, err := s.processTopicTransaction(ctx, data)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to process topic transaction")
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
		Stringer(logging.FieldBlockHash, block.Hash(s.config.ShardId)).
		Msg("Received block")

	lastBlock, lastHash, err := s.readLastBlock(ctx)
	if err != nil {
		return false, err
	}

	if block.Id != lastBlock.Id+1 {
		s.logger.Debug().
			Stringer(logging.FieldBlockNumber, block.Id).
			Msgf("Received block is out of order with the last block %d", lastBlock.Id)

		// todo: queue the block for later processing
		return false, nil
	}

	if block.PrevBlock != lastHash {
		txn := fmt.Sprintf("Prev block hash mismatch: expected %x, got %x", lastHash, block.PrevBlock)
		s.logger.Error().
			Stringer(logging.FieldBlockNumber, block.Id).
			Stringer(logging.FieldBlockHash, block.Hash(s.config.ShardId)).
			Msg(txn)
		panic(txn)
	}

	if err := s.saveBlock(ctx, b); err != nil {
		return false, err
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

	lastBlock, _, err := s.readLastBlock(ctx)
	if err != nil {
		return nil
	}

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
	if block == nil {
		return nil
	}

	// TODO: zerostate block is not signed and its hash should be checked in a bit different way
	// E.g. compare with network config value
	if block.Block.Id == 0 {
		return nil
	}

	if err := s.blockVerifier.VerifyBlock(ctx, s.logger, block.Block); err != nil {
		s.logger.Error().
			Uint64(logging.FieldBlockNumber, uint64(block.Id)).
			Stringer(logging.FieldBlockHash, block.Hash(s.config.ShardId)).
			Stringer(logging.FieldShardId, s.config.ShardId).
			Stringer(logging.FieldSignature, block.Signature).
			Err(err).
			Msg("Failed to verify block signature")
		return err
	}

	if s.config.ReplayBlocks {
		if err := s.replayBlock(ctx, block); err != nil {
			return err
		}
	} else {
		if err := s.saveDirectly(ctx, block); err != nil {
			return err
		}
	}
	s.notify()

	s.logger.Trace().
		Stringer(logging.FieldBlockNumber, block.Block.Id).
		Msg("Block written")

	return nil
}

func (s *Syncer) saveDirectly(ctx context.Context, block *types.BlockWithExtractedData) error {
	tx, err := s.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	blockHash := block.Block.Hash(s.config.ShardId)
	if err := db.WriteBlock(tx, s.config.ShardId, blockHash, block.Block); err != nil {
		return err
	}

	txnRoot, err := s.saveTransactions(tx, block.OutTransactions)
	if err != nil {
		return err
	}

	if txnRoot != block.Block.OutTransactionsRoot {
		transactionsJSON, err := json.Marshal(block.OutTransactions)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to marshal transactions")
			transactionsJSON = nil
		}
		blockJSON, err := json.Marshal(block.Block)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to marshal block")
			blockJSON = nil
		}
		s.logger.Debug().
			Stringer("expected", block.Block.OutTransactionsRoot).
			Stringer("got", txnRoot).
			RawJSON("transactions", transactionsJSON).
			RawJSON("block", blockJSON).
			Msg("Out transactions root mismatch")
		return fmt.Errorf("out transactions root mismatch. Expected %x, got %x",
			block.Block.OutTransactionsRoot, txnRoot)
	}

	_, err = execution.PostprocessBlock(tx, s.config.ShardId, block.Block.BaseFee, blockHash)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Syncer) GenerateZerostate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	if empty, err := s.shardIsEmpty(ctx); err != nil {
		return err
	} else if !empty {
		return nil
	}

	if len(s.config.BlockGeneratorParams.MainKeysOutPath) != 0 && s.config.ShardId == types.BaseShardId {
		if err := execution.DumpMainKeys(s.config.BlockGeneratorParams.MainKeysOutPath); err != nil {
			return err
		}
	}

	s.logger.Info().Msg("Generating zero-state...")

	gen, err := execution.NewBlockGenerator(ctx, s.config.BlockGeneratorParams, s.db, &common.EmptyHash)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	block, err := gen.GenerateZeroState(s.config.ZeroState, s.config.ZeroStateConfig)
	if err != nil {
		return err
	}

	return PublishBlock(ctx, s.networkManager, s.config.ShardId, &types.BlockWithExtractedData{Block: block})
}

func validateRepliedBlock(
	in, replied *types.Block, inHash, repliedHash common.Hash, inTxns, repliedTxns []*types.Transaction,
) error {
	if replied.OutTransactionsRoot != in.OutTransactionsRoot {
		return fmt.Errorf("out transactions root mismatch. Expected %x, got %x",
			in.OutTransactionsRoot, replied.OutTransactionsRoot)
	}
	if len(repliedTxns) != len(inTxns) {
		return fmt.Errorf("out transactions count mismatch. Expected %d, got %d",
			len(inTxns), len(repliedTxns))
	}
	if repliedHash != inHash {
		return fmt.Errorf("block hash mismatch. Expected %x, got %x",
			inHash, repliedHash)
	}
	return nil
}

func (s *Syncer) replayBlock(ctx context.Context, block *types.BlockWithExtractedData) error {
	mainShardHash := block.Block.MainChainHash
	if s.config.ShardId.IsMainShard() {
		mainShardHash = block.Block.PrevBlock
	}

	gen, err := execution.NewBlockGenerator(ctx, s.config.BlockGeneratorParams, s.db, &mainShardHash)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	blockHash := block.Block.Hash(s.config.ShardId)
	s.logger.Trace().
		Stringer(logging.FieldBlockNumber, block.Block.Id).
		Stringer(logging.FieldBlockHash, blockHash).
		Msg("Replaying block")

	proposal := &execution.Proposal{
		PrevBlockId:   block.Block.Id - 1,
		PrevBlockHash: block.Block.PrevBlock,
		MainChainHash: block.Block.MainChainHash,
		ShardHashes:   block.ChildBlocks,
	}
	proposal.InternalTxns, proposal.ExternalTxns = execution.SplitInTransactions(block.InTransactions)
	proposal.ForwardTxns, _ = execution.SplitOutTransactions(block.OutTransactions, s.config.ShardId)
	res, err := gen.GenerateBlock(proposal, s.logger, block.Signature)
	if err != nil {
		return err
	}

	if err := validateRepliedBlock(block.Block, res.Block, blockHash, res.Block.Hash(s.config.ShardId), block.OutTransactions, res.OutTxns); err != nil {
		return err
	}
	return nil
}

func (s *Syncer) saveTransactions(tx db.RwTx, transactions []*types.Transaction) (common.Hash, error) {
	transactionTree := execution.NewDbTransactionTrie(tx, s.config.ShardId)
	indexes := make([]types.TransactionIndex, len(transactions))
	for i := range transactions {
		indexes[i] = types.TransactionIndex(i)
	}
	if err := transactionTree.UpdateBatch(indexes, transactions); err != nil {
		return common.EmptyHash, err
	}
	return transactionTree.RootHash(), nil
}
