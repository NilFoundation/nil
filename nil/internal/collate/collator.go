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

var sharedLogger = logging.NewLogger("collator")

type collator struct {
	params Params

	topology ShardTopology
	pool     TxnPool

	logger zerolog.Logger

	proposal       *execution.Proposal
	executionState *execution.ExecutionState

	ctx  context.Context
	roTx db.RoTx
}

func newCollator(params Params, topology ShardTopology, pool TxnPool, logger zerolog.Logger) *collator {
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
	return &collator{
		params:   params,
		topology: topology,
		pool:     pool,
		logger:   logger,
	}
}

func (c *collator) GenerateProposal(ctx context.Context, txFabric db.DB) (*execution.Proposal, error) {
	c.proposal = execution.NewEmptyProposal()

	var err error
	c.roTx, err = txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer c.roTx.Rollback()

	configAccessor, err := config.NewConfigAccessor(ctx, txFabric, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}
	c.executionState, err = execution.NewExecutionState(c.roTx, c.params.ShardId, execution.StateParams{
		GetBlockFromDb: true,
		GasPriceScale:  c.params.GasPriceScale,
		ConfigAccessor: configAccessor,
	})
	if err != nil {
		return nil, err
	}

	c.executionState.UpdateGasPrice()

	c.logger.Trace().Msg("Collating...")

	if err := c.fetchPrevBlock(); err != nil {
		return nil, fmt.Errorf("failed to fetch previous block: %w", err)
	}

	if err := c.fetchLastBlockHashes(); err != nil {
		return nil, fmt.Errorf("failed to fetch last block hashes: %w", err)
	}

	if err := c.handleTransactionsFromNeighbors(); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from neighbors: %w", err)
	}

	if err := c.handleTransactionsFromPool(); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from pool: %w", err)
	}

	c.logger.Debug().Msgf("Collected %d in transactions (%d gas) and %d forward transactions",
		len(c.proposal.InTxns), c.executionState.GasUsed, len(c.proposal.ForwardTxns))

	return c.proposal, nil
}

func (c *collator) fetchPrevBlock() error {
	b, hash, err := db.ReadLastBlock(c.roTx, c.params.ShardId)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	c.proposal.PrevBlockId = b.Id
	c.proposal.PrevBlockHash = hash
	return nil
}

func (c *collator) fetchLastBlockHashes() error {
	if c.params.ShardId.IsMainShard() {
		c.proposal.ShardHashes = make([]common.Hash, c.params.NShards-1)
		for i := uint32(1); i < c.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(c.roTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			c.proposal.ShardHashes[i-1] = lastBlockHash
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(c.roTx, types.MainShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		c.proposal.MainChainHash = lastBlockHash
	}
	return nil
}

func (c *collator) handleTransaction(txn *types.Transaction, payer execution.Payer) error {
	if assert.Enable {
		txnHash := txn.Hash()
		defer func() {
			check.PanicIfNotf(txnHash == txn.Hash(), "Transaction hash changed during execution")
		}()
	}

	c.executionState.AddInTransaction(txn)

	res := c.executionState.HandleTransaction(c.ctx, txn, payer)
	if res.FatalError != nil {
		return res.FatalError
	} else if res.Failed() {
		c.logger.Debug().Stringer(logging.FieldTransactionHash, txn.Hash()).
			Err(res.Error).
			Msg("Transaction execution failed. It doesn't prevent adding it to the block.")
	}

	return nil
}

func (c *collator) handleTransactionsFromPool() error {
	poolTxns, err := c.pool.Peek(c.ctx, maxTxnsFromPool)
	if err != nil {
		return err
	}

	sa := execution.NewStateAccessor()

	handle := func(txn *types.Transaction) (bool, error) {
		hash := txn.Hash()

		if txnData, err := sa.Access(c.roTx, c.params.ShardId).GetInTransaction().ByHash(hash); err != nil &&
			!errors.Is(err, db.ErrKeyNotFound) {
			return false, err
		} else if err == nil && txnData.Transaction() != nil {
			c.logger.Trace().Stringer(logging.FieldTransactionHash, hash).
				Msg("Transaction is already in the blockchain. Dropping...")
			return false, nil
		}

		if res := execution.ValidateExternalTransaction(c.executionState, txn); res.FatalError != nil {
			return false, res.FatalError
		} else if res.Failed() {
			execution.AddFailureReceipt(hash, txn.To, res)
			return false, nil
		}

		acc, err := c.executionState.GetAccount(txn.To)
		if err != nil {
			return false, err
		}

		if err := c.handleTransaction(txn, execution.NewAccountPayer(acc, txn)); err != nil {
			return false, err
		}

		return true, nil
	}

	for _, txn := range poolTxns {
		if ok, err := handle(txn); err != nil {
			return err
		} else if ok {
			if c.executionState.GasUsed > c.params.MaxGasInBlock {
				break
			}

			c.proposal.InTxns = append(c.proposal.InTxns, txn)
		}

		c.proposal.RemoveFromPool = append(c.proposal.RemoveFromPool, txn)
	}

	return nil
}

func (c *collator) handleTransactionsFromNeighbors() error {
	state, err := db.ReadCollatorState(c.roTx, c.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	neighborIndexes := common.SliceToMap(state.Neighbors, func(i int, t types.Neighbor) (types.ShardId, int) {
		return t.ShardId, i
	})

	checkLimits := func() bool {
		return c.executionState.GasUsed < c.params.MaxInternalGasInBlock &&
			len(c.proposal.InTxns) < c.params.MaxInternalTransactionsInBlock &&
			len(c.proposal.ForwardTxns) < c.params.MaxForwardTransactionsInBlock
	}

	for _, neighborId := range c.topology.GetNeighbors(c.params.ShardId, c.params.NShards, true) {
		position, ok := neighborIndexes[neighborId]
		if !ok {
			position = len(neighborIndexes)
			neighborIndexes[neighborId] = position
			state.Neighbors = append(state.Neighbors, types.Neighbor{ShardId: neighborId})
		}
		neighbor := &state.Neighbors[position]

		var lastBlockNumber types.BlockNumber
		lastBlock, _, err := db.ReadLastBlock(c.roTx, neighborId)
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
			block, err := db.ReadBlockByNumber(c.roTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outTxnTrie := execution.NewDbTransactionTrieReader(c.roTx, neighborId)
			outTxnTrie.SetRootHash(block.OutTransactionsRoot)
			for ; neighbor.TransactionIndex < block.OutTransactionsNum; neighbor.TransactionIndex++ {
				txn, err := outTxnTrie.Fetch(neighbor.TransactionIndex)
				if err != nil {
					return err
				}

				if txn.To.ShardId() == c.params.ShardId {
					if err := execution.ValidateInternalTransaction(txn); err != nil {
						c.logger.Warn().Err(err).Msg("Invalid internal transaction")
					} else {
						if err := c.handleTransaction(txn, execution.NewTransactionPayer(txn, c.executionState)); err != nil {
							return err
						}

						if !checkLimits() {
							break
						}
					}

					c.proposal.InTxns = append(c.proposal.InTxns, txn)
				} else if c.params.ShardId != neighborId {
					if c.topology.ShouldPropagateTxn(neighborId, c.params.ShardId, txn.To.ShardId()) {
						if !checkLimits() {
							break
						}

						c.proposal.ForwardTxns = append(c.proposal.ForwardTxns, txn)
					}
				}
			}

			if neighbor.TransactionIndex == block.OutTransactionsNum {
				neighbor.BlockNumber++
				neighbor.TransactionIndex = 0
			}
		}
	}

	c.logger.Debug().Msgf("Collected %d incoming transactions from neigbors with %d gas",
		len(c.proposal.InTxns), c.executionState.GasUsed)

	c.proposal.CollatorState = state
	return nil
}
