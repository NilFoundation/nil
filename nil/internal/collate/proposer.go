package collate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

const (
	defaultMaxInternalTxns               = 1000
	defaultMaxInternalGasInBlock         = 100_000_000
	defaultMaxGasInBlock                 = 2 * defaultMaxInternalGasInBlock
	maxTxnsFromPool                      = 1000
	defaultMaxForwardTransactionsInBlock = 200
)

type proposer struct {
	params Params

	topology ShardTopology
	pool     TxnPool

	logger zerolog.Logger

	proposal       *execution.Proposal
	executionState *execution.ExecutionState

	ctx  context.Context
	roTx db.RoTx
}

func newProposer(params Params, topology ShardTopology, pool TxnPool, logger zerolog.Logger) *proposer {
	if params.MaxGasInBlock == 0 {
		params.MaxGasInBlock = defaultMaxGasInBlock
	}
	if params.MaxInternalGasInBlock == 0 {
		params.MaxInternalGasInBlock = defaultMaxInternalGasInBlock
	}
	if params.MaxInternalGasInBlock > params.MaxGasInBlock {
		params.MaxInternalGasInBlock = params.MaxGasInBlock
	}
	if params.MaxInternalTransactionsInBlock == 0 {
		params.MaxInternalTransactionsInBlock = defaultMaxInternalTxns
	}
	if params.MaxForwardTransactionsInBlock == 0 {
		params.MaxForwardTransactionsInBlock = defaultMaxForwardTransactionsInBlock
	}
	return &proposer{
		params:   params,
		topology: topology,
		pool:     pool,
		logger:   logger,
	}
}

func (p *proposer) GenerateProposal(ctx context.Context, txFabric db.DB) (*execution.Proposal, error) {
	p.proposal = execution.NewEmptyProposal()

	var err error
	p.roTx, err = txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer p.roTx.Rollback()

	configAccessor, err := config.NewConfigAccessor(ctx, txFabric, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}
	p.executionState, err = execution.NewExecutionState(p.roTx, p.params.ShardId, execution.StateParams{
		GetBlockFromDb: true,
		GasPriceScale:  p.params.GasPriceScale,
		ConfigAccessor: configAccessor,
	})
	if err != nil {
		return nil, err
	}

	if err = p.executionState.UpdateBaseFee(); err != nil {
		return nil, fmt.Errorf("failed to update gas price: %w", err)
	}

	p.logger.Trace().Msg("Collating...")

	if err := p.fetchPrevBlock(); err != nil {
		return nil, fmt.Errorf("failed to fetch previous block: %w", err)
	}

	if err := p.fetchLastBlockHashes(); err != nil {
		return nil, fmt.Errorf("failed to fetch last block hashes: %w", err)
	}

	if err := p.handleTransactionsFromNeighbors(); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from neighbors: %w", err)
	}

	if err := p.handleTransactionsFromPool(); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from pool: %w", err)
	}

	p.logger.Debug().Msgf("Collected %d in transactions (%d gas) and %d forward transactions",
		len(p.proposal.InTxns), p.executionState.GasUsed, len(p.proposal.ForwardTxns))

	return p.proposal, nil
}

func (p *proposer) fetchPrevBlock() error {
	b, hash, err := db.ReadLastBlock(p.roTx, p.params.ShardId)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	p.proposal.PrevBlockId = b.Id
	p.proposal.PrevBlockHash = hash
	return nil
}

func (p *proposer) fetchLastBlockHashes() error {
	if p.params.ShardId.IsMainShard() {
		p.proposal.ShardHashes = make([]common.Hash, p.params.NShards-1)
		for i := uint32(1); i < p.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(p.roTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			p.proposal.ShardHashes[i-1] = lastBlockHash
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(p.roTx, types.MainShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		p.proposal.MainChainHash = lastBlockHash
	}
	return nil
}

func (p *proposer) handleTransaction(txn *types.Transaction, payer execution.Payer) error {
	if assert.Enable {
		txnHash := txn.Hash()
		defer func() {
			check.PanicIfNotf(txnHash == txn.Hash(), "Transaction hash changed during execution")
		}()
	}

	p.executionState.AddInTransaction(txn)

	res := p.executionState.HandleTransaction(p.ctx, txn, payer)
	if res.FatalError != nil {
		return res.FatalError
	} else if res.Failed() {
		p.logger.Debug().Stringer(logging.FieldTransactionHash, txn.Hash()).
			Err(res.Error).
			Msg("Transaction execution failed. It doesn't prevent adding it to the block.")
	}

	return nil
}

func (p *proposer) handleTransactionsFromPool() error {
	poolTxns, err := p.pool.Peek(p.ctx, maxTxnsFromPool)
	if err != nil {
		return err
	}

	sa := execution.NewStateAccessor()

	handle := func(txn *types.Transaction) (bool, error) {
		hash := txn.Hash()

		if txnData, err := sa.Access(p.roTx, p.params.ShardId).GetInTransaction().ByHash(hash); err != nil &&
			!errors.Is(err, db.ErrKeyNotFound) {
			return false, err
		} else if err == nil && txnData.Transaction() != nil {
			p.logger.Trace().Stringer(logging.FieldTransactionHash, hash).
				Msg("Transaction is already in the blockchain. Dropping...")
			return false, nil
		}

		if res := execution.ValidateExternalTransaction(p.executionState, txn); res.FatalError != nil {
			return false, res.FatalError
		} else if res.Failed() {
			p.logger.Error().Stringer(logging.FieldTransactionHash, hash).
				Err(res.Error).Msg("External message validation failed")
			execution.AddFailureReceipt(hash, txn.To, res)
			return false, nil
		}

		acc, err := p.executionState.GetAccount(txn.To)
		if err != nil {
			return false, err
		}

		if err := p.handleTransaction(txn, execution.NewAccountPayer(acc, txn)); err != nil {
			return false, err
		}

		return true, nil
	}

	for _, txn := range poolTxns {
		if ok, err := handle(txn); err != nil {
			return err
		} else if ok {
			if p.executionState.GasUsed > p.params.MaxGasInBlock {
				break
			}

			p.proposal.InTxns = append(p.proposal.InTxns, txn)
		}

		p.proposal.RemoveFromPool = append(p.proposal.RemoveFromPool, txn)
	}

	return nil
}

func (p *proposer) handleTransactionsFromNeighbors() error {
	state, err := db.ReadCollatorState(p.roTx, p.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	neighborIndexes := common.SliceToMap(state.Neighbors, func(i int, t types.Neighbor) (types.ShardId, int) {
		return t.ShardId, i
	})

	checkLimits := func() bool {
		return p.executionState.GasUsed < p.params.MaxInternalGasInBlock &&
			len(p.proposal.InTxns) < p.params.MaxInternalTransactionsInBlock &&
			len(p.proposal.ForwardTxns) < p.params.MaxForwardTransactionsInBlock
	}

	for _, neighborId := range p.topology.GetNeighbors(p.params.ShardId, p.params.NShards, true) {
		position, ok := neighborIndexes[neighborId]
		if !ok {
			position = len(neighborIndexes)
			neighborIndexes[neighborId] = position
			state.Neighbors = append(state.Neighbors, types.Neighbor{ShardId: neighborId})
		}
		neighbor := &state.Neighbors[position]

		var lastBlockNumber types.BlockNumber
		lastBlock, _, err := db.ReadLastBlock(p.roTx, neighborId)
		if !errors.Is(err, db.ErrKeyNotFound) {
			if err != nil {
				return err
			}
			lastBlockNumber = lastBlock.Id
		}

		for checkLimits() {
			// We will break the loop when lastBlockNumber is reached anyway,
			// but in case of read-through mode, we will make unnecessary requests to the server if we don't check it here.
			if lastBlockNumber < neighbor.BlockNumber {
				break
			}
			block, err := db.ReadBlockByNumber(p.roTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outTxnTrie := execution.NewDbTransactionTrieReader(p.roTx, neighborId)
			outTxnTrie.SetRootHash(block.OutTransactionsRoot)
			for ; neighbor.TransactionIndex < block.OutTransactionsNum; neighbor.TransactionIndex++ {
				txn, err := outTxnTrie.Fetch(neighbor.TransactionIndex)
				if err != nil {
					return err
				}

				if txn.To.ShardId() == p.params.ShardId {
					if err := execution.ValidateInternalTransaction(txn); err != nil {
						p.logger.Warn().Err(err).Msg("Invalid internal transaction")
					} else {
						if err := p.handleTransaction(txn, execution.NewTransactionPayer(txn, p.executionState)); err != nil {
							return err
						}
					}
					p.proposal.InTxns = append(p.proposal.InTxns, txn)

					// Handle at least one transaction.
					if len(p.proposal.InTxns) >= 1 {
						if !checkLimits() {
							break
						}
					}
				} else if p.params.ShardId != neighborId {
					if p.topology.ShouldPropagateTxn(neighborId, p.params.ShardId, txn.To.ShardId()) {
						if !checkLimits() {
							break
						}

						p.proposal.ForwardTxns = append(p.proposal.ForwardTxns, txn)
					}
				}
			}

			if neighbor.TransactionIndex == block.OutTransactionsNum {
				neighbor.BlockNumber++
				neighbor.TransactionIndex = 0
			}
		}
	}

	p.logger.Debug().Msgf("Collected %d incoming transactions from neigbors with %d gas",
		len(p.proposal.InTxns), p.executionState.GasUsed)

	p.proposal.CollatorState = state
	return nil
}
