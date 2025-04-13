package collate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	cerrors "github.com/NilFoundation/nil/nil/internal/collate/errors"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/signer"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type invalidSignatureError struct {
	inner error
}

func (e invalidSignatureError) Error() string {
	return fmt.Sprintf("invalid block signature: %v", e.inner)
}

func newErrInvalidSignature(inner error) invalidSignatureError {
	return invalidSignatureError{inner: inner}
}

type eventType int

const (
	/* Block inserted by validator. */
	commitType eventType = iota
	/* Block inserted by syncer. */
	replayType
)

type event struct {
	evType      eventType
	blockNumber types.BlockNumber
}

type Validator struct {
	params             *Params
	mainShardValidator *Validator // +checklocksignore: thread safe

	txFabric       db.DB
	pool           TxnPool
	networkManager network.Manager       // +checklocksignore: thread safe
	blockVerifier  *signer.BlockVerifier // +checklocksignore: thread safe

	mutex         sync.RWMutex
	lastBlock     *types.Block    // +checklocks:mutex
	lastBlockHash common.Hash     // +checklocks:mutex
	metrics       *MetricsHandler // +checklocks:mutex

	subsMutex sync.Mutex
	subsId    uint64                 // +checklocks:subsMutex
	subs      map[uint64]chan *event // +checklocks:subsMutex

	logger logging.Logger
}

func NewValidator(
	params *Params, mainShardValidator *Validator, txFabric db.DB, pool TxnPool, nm network.Manager,
) (*Validator, error) {
	metrics, err := NewMetricsHandler(params.ShardId)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics handler: %w", err)
	}

	return &Validator{
		params:             params,
		mainShardValidator: mainShardValidator,
		txFabric:           txFabric,
		pool:               pool,
		networkManager:     nm,
		blockVerifier:      signer.NewBlockVerifier(params.ShardId, txFabric),
		subs:               make(map[uint64]chan *event),
		logger: logging.NewLogger("validator").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
		metrics: metrics,
	}, nil
}

// +checklocksread:s.mutex
func (s *Validator) getLastBlockUnlocked(ctx context.Context) (*types.Block, common.Hash, error) {
	if s.lastBlock != nil {
		return s.lastBlock, s.lastBlockHash, nil
	}

	rotx, err := s.txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, common.EmptyHash, err
	}
	defer rotx.Rollback()

	block, hash, err := db.ReadLastBlock(rotx, s.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, common.EmptyHash, err
	}
	if err == nil {
		return block, hash, err
	}
	return nil, common.EmptyHash, nil
}

func (s *Validator) GetLastBlock(ctx context.Context) (*types.Block, common.Hash, error) {
	s.mutex.RLock()
	lastBlock, lastBlockHash := s.lastBlock, s.lastBlockHash
	s.mutex.RUnlock()

	if lastBlock != nil {
		return lastBlock, lastBlockHash, nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.getLastBlockUnlocked(ctx)
}

func (s *Validator) getBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	tx, err := s.txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := db.ReadBlock(tx, s.params.ShardId, hash)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (s *Validator) TxPool() TxnPool {
	return s.pool
}

func (s *Validator) BuildProposal(ctx context.Context) (*execution.ProposalSSZ, error) {
	// No lock since it doesn't directly access last block/hash
	proposer := newProposer(s.params, s.params.Topology, s.pool, s.logger)
	proposal, err := proposer.GenerateProposal(ctx, s.txFabric)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proposal: %w", err)
	}

	p, err := execution.ConvertProposal(proposal)
	if err != nil {
		return nil, err
	}

	hash, err := s.buildBlockHashByProposal(ctx, p)
	if err != nil {
		return nil, err
	}
	proposal.BlockHash = hash

	return proposal, nil
}

func (s *Validator) buildBlockHashByProposal(ctx context.Context, proposal *execution.Proposal) (common.Hash, error) {
	prevBlock, err := s.getBlock(ctx, proposal.PrevBlockHash)
	if err != nil {
		return common.EmptyHash, err
	}

	params := s.params.BlockGeneratorParams
	params.ExecutionMode = execution.ModeVerify
	gen, err := execution.NewBlockGenerator(ctx, params, s.txFabric, prevBlock)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	gasPrices := gen.CollectGasPrices(proposal.PrevBlockId)
	res, err := gen.BuildBlock(proposal, gasPrices)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to generate block: %w", err)
	}
	return res.BlockHash, nil
}

func (s *Validator) IsValidProposal(ctx context.Context, proposal *execution.ProposalSSZ) error {
	p, err := execution.ConvertProposal(proposal)
	if err != nil {
		return err
	}

	// No lock since below we use only locked functions and only in read mode
	if err := s.validateProposal(ctx, p); err != nil {
		return err
	}

	hash, err := s.buildBlockHashByProposal(ctx, p)
	if err != nil {
		return fmt.Errorf("failed to build block by proposal: %w", err)
	}

	if proposal.BlockHash != hash {
		s.logger.Error().
			Stringer("proposedHash", proposal.BlockHash).
			Stringer("gotHash", hash).
			Err(cerrors.ErrInvalidProposedHash).
			Msg("proposed block hash is different")
		return cerrors.ErrInvalidProposedHash
	}
	return nil
}

func (s *Validator) InsertProposal(
	ctx context.Context,
	proposal *execution.ProposalSSZ,
	params *types.ConsensusParams,
) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.insertProposalUnlocked(ctx, proposal, params)
}

// +checklocks:s.mutex
func (s *Validator) insertProposalUnlocked(
	ctx context.Context,
	proposal *execution.ProposalSSZ,
	consensusParams *types.ConsensusParams,
) error {
	p, err := execution.ConvertProposal(proposal)
	if err != nil {
		return err
	}
	if err := s.validateProposalUnlocked(ctx, p); err != nil {
		return err
	}

	prevBlock, err := s.getBlock(ctx, proposal.PrevBlockHash)
	if err != nil {
		return err
	}

	params := s.params.BlockGeneratorParams
	params.ExecutionMode = execution.ModeProposal
	gen, err := execution.NewBlockGenerator(ctx, params, s.txFabric, prevBlock)
	if err != nil {
		return fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	s.metrics.StartProcessingMeasurement()
	res, err := gen.GenerateBlock(p, consensusParams)
	if err != nil {
		return fmt.Errorf("failed to generate block: %w", err)
	}
	s.metrics.EndProcessingMeasurement(ctx, res.Counters)

	s.onBlockCommitUnlocked(ctx, res, p, commitType)

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &types.BlockWithExtractedData{
		Block:           res.Block,
		InTransactions:  res.InTxns,
		OutTransactions: res.OutTxns,
		ChildBlocks:     proposal.ShardHashes,
		Config: common.TransformMap(res.ConfigParams, func(k string, v []byte) (string, hexutil.Bytes) {
			return k, v
		}),
	})
}

// +checklocks:s.mutex
func (s *Validator) onBlockCommitUnlocked(
	ctx context.Context, res *execution.BlockGenerationResult, proposal *execution.Proposal, evType eventType,
) {
	s.setLastBlockUnlocked(res.Block, res.BlockHash)

	s.metrics.RecordBlockId(ctx, res.Block.Id)

	if !reflect.ValueOf(s.pool).IsNil() {
		if err := s.pool.OnCommitted(ctx, res.Block.BaseFee, proposal.ExternalTxns); err != nil {
			s.logger.Warn().Err(err).
				Msgf("Failed to remove %d committed transactions from pool", len(proposal.ExternalTxns))
		}
	}

	s.notify(&event{evType, res.Block.Id})
}

func (s *Validator) logBlockDiffError(expected, got *types.Block, expHash, gotHash common.Hash) error {
	msg := fmt.Sprintf("block hash mismatch: expected %x, got %x", expHash, gotHash)
	blockMarshal, err := json.Marshal(got)
	check.PanicIfErr(err)
	lastBlockMarshal, err := json.Marshal(expected)
	check.PanicIfErr(err)
	s.logger.Error().
		Stringer(logging.FieldBlockNumber, expected.Id).
		Stringer("expectedHash", expHash).
		Stringer("gotHash", gotHash).
		RawJSON("expected", blockMarshal).
		RawJSON("got", lastBlockMarshal).
		Msg(msg)
	return returnErrorOrPanic(errors.New(msg))
}

func (s *Validator) validateRepliedBlock(
	in *types.BlockWithExtractedData, replied *execution.BlockGenerationResult,
	inHash common.Hash, inTxns []*types.Transaction,
) error {
	if replied.Block.OutTransactionsRoot != in.OutTransactionsRoot {
		return returnErrorOrPanic(fmt.Errorf("out transactions root mismatch. Expected %x, got %x",
			in.OutTransactionsRoot, replied.Block.OutTransactionsRoot))
	}
	if len(replied.OutTxns) != len(inTxns) {
		return returnErrorOrPanic(fmt.Errorf("out transactions count mismatch. Expected %d, got %d",
			len(inTxns), len(replied.InTxns)))
	}
	if replied.Block.ConfigRoot != in.ConfigRoot {
		expectedConfigJson, err := json.Marshal(in.Config)
		check.PanicIfErr(err)

		gotConfig := common.TransformMap(replied.ConfigParams, func(k string, v []byte) (string, hexutil.Bytes) {
			return k, hexutil.Bytes(v)
		})
		gotConfigJson, err := json.Marshal(gotConfig)
		check.PanicIfErr(err)

		err = fmt.Errorf("config root mismatch. Expected %x, got %x", in.ConfigRoot, replied.Block.ConfigRoot)
		s.logger.Error().Err(err).
			RawJSON("expectedConfig", expectedConfigJson).
			RawJSON("gotConfig", gotConfigJson).
			Msg("config root mismatch")
		return returnErrorOrPanic(err)
	}
	if replied.BlockHash != inHash {
		return s.logBlockDiffError(in.Block, replied.Block, inHash, replied.BlockHash)
	}
	return nil
}

// +checklocksread:s.mutex
func (s *Validator) validateBlockForProposalUnlocked(ctx context.Context, block *types.BlockWithExtractedData) error {
	proposal := &execution.Proposal{
		PrevBlockId:   block.Id - 1,
		PrevBlockHash: block.PrevBlock,
		MainShardHash: block.MainShardHash,
		ShardHashes:   block.ChildBlocks,
	}
	return s.validateProposalUnlocked(ctx, proposal)
}

// +checklocksread:s.mutex
func (s *Validator) validateProposalUnlocked(ctx context.Context, proposal *execution.Proposal) error {
	lastBlock, lastBlockHash, err := s.getLastBlockUnlocked(ctx)
	if err != nil {
		return err
	}

	blockId := proposal.PrevBlockId + 1
	if blockId <= lastBlock.Id {
		s.logger.Trace().
			Err(cerrors.ErrOldBlock).
			Stringer(logging.FieldBlockNumber, blockId).
			Send()
		return cerrors.ErrOldBlock
	}

	if blockId != lastBlock.Id+1 {
		s.logger.Debug().
			Stringer(logging.FieldBlockNumber, blockId).
			Msgf("Received block %d is out of order with the last block %d", blockId, lastBlock.Id)
		return cerrors.ErrOutOfOrder
	}

	if lastBlockHash != proposal.PrevBlockHash {
		lastBlockMarshal, err := json.Marshal(lastBlock)
		check.PanicIfErr(err)

		s.logger.Error().
			RawJSON("lastBlock", lastBlockMarshal).
			Stringer("lastHash", lastBlockHash).
			Stringer("expectedLastHash", proposal.PrevBlockHash).
			Msgf("Previous block hash mismatch: expected %x, got %x", lastBlockHash, proposal.PrevBlockHash)
		return cerrors.ErrHashMismatch
	}

	return nil
}

func (s *Validator) validateProposal(ctx context.Context, proposal *execution.Proposal) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.validateProposalUnlocked(ctx, proposal)
}

func (s *Validator) ReplayBlock(ctx context.Context, block *types.BlockWithExtractedData) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.replayBlockUnlocked(ctx, block)
}

// +checklocks:s.mutex
func (s *Validator) replayBlockUnlocked(ctx context.Context, block *types.BlockWithExtractedData) error {
	blockHash := block.Hash(s.params.ShardId)
	s.logger.Trace().
		Stringer(logging.FieldBlockNumber, block.Id).
		Stringer(logging.FieldBlockHash, blockHash).
		Msg("Replaying block")

	if err := s.validateBlockForProposalUnlocked(ctx, block); err != nil {
		if errors.Is(err, cerrors.ErrHashMismatch) {
			return returnErrorOrPanic(err)
		}
		return err
	}

	proposal := &execution.Proposal{
		PrevBlockId:   block.Id - 1,
		PrevBlockHash: block.PrevBlock,
		MainShardHash: block.MainShardHash,
		ShardHashes:   block.ChildBlocks,
	}
	proposal.InternalTxns, proposal.ExternalTxns = execution.SplitInTransactions(block.InTransactions)
	proposal.ForwardTxns, _ = execution.SplitOutTransactions(block.OutTransactions, s.params.ShardId)

	var gasPrices []types.Uint256
	if s.params.ShardId.IsMainShard() {
		if gasPricesBytes, ok := block.Config[config.NameGasPrice]; ok {
			param := &config.ParamGasPrice{}
			if err := param.UnmarshalSSZ(gasPricesBytes); err != nil {
				return fmt.Errorf("failed to unmarshal gas prices: %w", err)
			}
			gasPrices = param.Shards
		}
	}

	prevBlock, _, err := s.getLastBlockUnlocked(ctx)
	if err != nil {
		return err
	}

	if !s.params.ShardId.IsMainShard() {
		// To verify/execute block properly we need to be sure that we have an access to config.
		// Config for block N is stored inside main shard block for block N-1.
		// So we need to wait until config is available.
		if err := s.mainShardValidator.WaitForBlock(ctx, prevBlock.MainShardHash); err != nil {
			return fmt.Errorf("failed to wait for main shard block: %w", err)
		}
	}

	if !s.params.DisableConsensus {
		if err := s.blockVerifier.VerifyBlock(ctx, block.Block); err != nil {
			s.logger.Error().
				Uint64(logging.FieldBlockNumber, uint64(block.Id)).
				Stringer(logging.FieldBlockHash, block.Hash(s.params.ShardId)).
				Stringer(logging.FieldSignature, block.Signature).
				Err(err).
				Msg("Failed to verify block signature")
			return newErrInvalidSignature(err)
		}
	}

	params := s.params.BlockGeneratorParams
	params.ExecutionMode = execution.ModeSyncReplay
	gen, err := execution.NewBlockGenerator(ctx, params, s.txFabric, prevBlock)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	// First we build block without writing it into the database, because we need to check that the resulted block is
	// the same as the proposed one.
	resBlock, err := gen.BuildBlock(proposal, gasPrices)
	if err != nil {
		return fmt.Errorf("failed to build block: %w", err)
	}

	// Check generated block and proposed are equal
	if err = s.validateRepliedBlock(block, resBlock, blockHash, block.OutTransactions); err != nil {
		return fmt.Errorf("failed to validate replied block: %w", err)
	}

	// Finally, write generated block into the database
	if err = gen.Finalize(resBlock, &block.ConsensusParams); err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	s.onBlockCommitUnlocked(ctx, resBlock, proposal, replayType)

	return nil
}

// +checklocks:s.mutex
func (s *Validator) setLastBlockUnlocked(block *types.Block, hash common.Hash) {
	s.lastBlock = block
	s.lastBlockHash = hash
}

func (s *Validator) Subscribe() (uint64, <-chan *event) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	ch := make(chan *event, 1)
	id := s.subsId
	s.subs[id] = ch
	s.subsId++
	return id, ch
}

func (s *Validator) Unsubscribe(id uint64) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	close(s.subs[id])
	delete(s.subs, id)
}

func (s *Validator) notify(ev *event) {
	s.subsMutex.Lock()
	defer s.subsMutex.Unlock()

	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *Validator) checkBlock(ctx context.Context, expectedHash common.Hash) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.checkBlockUnlocked(ctx, expectedHash)
}

// +checklocksread:s.mutex
func (s *Validator) checkBlockUnlocked(ctx context.Context, expectedHash common.Hash) (bool, error) {
	_, hash, err := s.getLastBlockUnlocked(ctx)
	if err != nil {
		return false, err
	}

	// Fast path
	if hash == expectedHash {
		return true, nil
	}

	// Slow path with block lookup from DB
	block, err := s.getBlock(ctx, expectedHash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return false, nil
	}
	return block != nil, err
}

func (s *Validator) WaitForBlock(ctx context.Context, expectedHash common.Hash) error {
	s.mutex.RLock()
	ok, err := s.checkBlockUnlocked(ctx, expectedHash)
	if err != nil || ok {
		s.mutex.RUnlock()
		return err
	}

	subId, subChan := s.Subscribe()
	defer s.Unsubscribe(subId)
	s.mutex.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-subChan:
			ok, err := s.checkBlock(ctx, expectedHash)
			if err != nil || ok {
				return err
			}
		case <-time.After(s.params.Timeout):
			ok, err := s.checkBlock(ctx, expectedHash)
			if err != nil || ok {
				return err
			}
			return fmt.Errorf("timeout waiting for block %x", expectedHash)
		}
	}
}
