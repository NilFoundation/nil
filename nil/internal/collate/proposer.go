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
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rollup"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	l1types "github.com/ethereum/go-ethereum/core/types"
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

	ctx context.Context

	l1BlockFetcher rollup.L1BlockFetcher
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
		params:         params,
		topology:       topology,
		pool:           pool,
		logger:         logger,
		l1BlockFetcher: params.L1Fetcher,
	}
}

func (p *proposer) GenerateProposal(ctx context.Context, txFabric db.DB) (*execution.Proposal, error) {
	p.proposal = execution.NewEmptyProposal()

	tx, err := txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, err := p.fetchPrevBlock(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch previous block: %w", err)
	}

	configAccessor, err := config.NewConfigAccessorFromBlockWithTx(tx, block, p.params.ShardId)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	p.executionState, err = execution.NewExecutionState(tx, p.params.ShardId, execution.StateParams{
		Block:          block,
		ConfigAccessor: configAccessor,
	})
	if err != nil {
		return nil, err
	}

	if err = p.executionState.UpdateBaseFee(block); err != nil {
		return nil, fmt.Errorf("failed to update gas price: %w", err)
	}

	p.logger.Trace().Msg("Collating...")

	if err := p.fetchLastBlockHashes(tx); err != nil {
		return nil, fmt.Errorf("failed to fetch last block hashes: %w", err)
	}

	if err := p.handleL1Attributes(tx); err != nil {
		// TODO: change to Error severity once Consensus/Proposer increase time intervals
		p.logger.Trace().Err(err).Msg("Failed to handle L1 attributes")
	}

	if err := p.handleTransactionsFromNeighbors(tx); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from neighbors: %w", err)
	}

	if err := p.handleTransactionsFromPool(tx); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from pool: %w", err)
	}

	p.logger.Debug().Msgf("Collected %d internal, %d external (%d gas) and %d forward transactions",
		len(p.proposal.InternalTxns), len(p.proposal.ExternalTxns), p.executionState.GasUsed, len(p.proposal.ForwardTxns))

	return p.proposal, nil
}

func (p *proposer) fetchPrevBlock(tx db.RoTx) (*types.Block, error) {
	b, hash, err := db.ReadLastBlock(tx, p.params.ShardId)
	if err != nil {
		return nil, err
	}

	p.proposal.PrevBlockId = b.Id
	p.proposal.PrevBlockHash = hash
	return b, nil
}

func (p *proposer) fetchLastBlockHashes(tx db.RoTx) error {
	if p.params.ShardId.IsMainShard() {
		p.proposal.ShardHashes = make([]common.Hash, p.params.NShards-1)
		for i := uint32(1); i < p.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			p.proposal.ShardHashes[i-1] = lastBlockHash
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(tx, types.MainShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}
		p.proposal.MainChainHash = lastBlockHash
	}

	return nil
}

func (p *proposer) handleL1Attributes(tx db.RoTx) error {
	if !p.params.ShardId.IsMainShard() {
		return nil
	}
	if p.l1BlockFetcher == nil {
		return errors.New("L1 block fetcher is not initialized")
	}

	block, err := p.l1BlockFetcher.GetLastBlockInfo(p.ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest L1 block: %w", err)
	}
	if block == nil {
		// No block yet
		return nil
	}

	// Check if this L1 block was already processed
	if cfgAccessor, err := config.NewConfigReader(tx, nil); err == nil {
		if prevL1Block, err := config.GetParamL1Block(cfgAccessor); err == nil {
			if prevL1Block != nil && prevL1Block.Number >= block.Number.Uint64() {
				return nil
			}
		}
	}

	txn, err := CreateL1BlockUpdateTransaction(block)
	if err != nil {
		return fmt.Errorf("failed to create L1 block update transaction: %w", err)
	}

	p.logger.Debug().
		Stringer("hash", txn.Hash()).
		Stringer("block_num", block.Number).
		Stringer("base_fee", block.BaseFee).
		Msg("Add L1 block update transaction")

	p.proposal.InternalTxns = append(p.proposal.InternalTxns, txn)

	return nil
}

func CreateL1BlockUpdateTransaction(header *l1types.Header) (*types.Transaction, error) {
	abi, err := contracts.GetAbi(contracts.NameL1BlockInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get L1BlockInfo ABI: %w", err)
	}

	blobBaseFee, err := rollup.GetBlobGasPrice(header)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate blob base fee: %w", err)
	}

	calldata, err := abi.Pack("setL1BlockInfo",
		header.Number.Uint64(),
		header.Time,
		header.BaseFee,
		blobBaseFee.ToBig(),
		header.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to pack setL1BlockInfo calldata: %w", err)
	}

	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:                types.NewTransactionFlags(types.TransactionFlagInternal),
			To:                   types.L1BlockInfoAddress,
			FeeCredit:            types.GasToValue(types.DefaultGasLimit.Uint64()),
			MaxFeePerGas:         types.MaxFeePerGasDefault,
			MaxPriorityFeePerGas: types.Value0,
			Data:                 calldata,
		},
		From: types.L1BlockInfoAddress,
	}

	return txn, nil
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

func (p *proposer) handleTransactionsFromPool(tx db.RoTx) error {
	poolTxns, err := p.pool.Peek(p.ctx, maxTxnsFromPool)
	if err != nil {
		return err
	}

	sa := execution.NewStateAccessor()

	var duplicates, unverified []*types.Transaction
	handle := func(txn *types.Transaction) (bool, error) {
		hash := txn.Hash()

		if txnData, err := sa.Access(tx, p.params.ShardId).GetInTransaction().ByHash(hash); err != nil &&
			!errors.Is(err, db.ErrKeyNotFound) {
			return false, err
		} else if err == nil && txnData.Transaction() != nil {
			p.logger.Trace().Stringer(logging.FieldTransactionHash, hash).
				Msg("Transaction is already in the blockchain. Dropping...")

			duplicates = append(duplicates, txn)
			return false, nil
		}

		if res := execution.ValidateExternalTransaction(p.executionState, txn); res.FatalError != nil {
			return false, res.FatalError
		} else if res.Failed() {
			p.logger.Info().Stringer(logging.FieldTransactionHash, hash).
				Err(res.Error).Msg("External txn validation failed. Saved failure receipt. Dropping...")

			execution.AddFailureReceipt(hash, txn.To, res)
			unverified = append(unverified, txn)
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

			p.proposal.ExternalTxns = append(p.proposal.ExternalTxns, txn)
		}
	}

	if len(duplicates) > 0 {
		p.logger.Debug().Msgf("Removing %d duplicate transactions from the pool", len(duplicates))

		if err := p.pool.Discard(p.ctx, duplicates, txnpool.DuplicateHash); err != nil {
			p.logger.Error().Err(err).
				Msgf("Failed to remove %d duplicate transactions from the pool", len(duplicates))
		}
	}

	if len(unverified) > 0 {
		p.logger.Debug().Msgf("Removing %d unverifiable transactions from the pool", len(unverified))

		if err := p.pool.Discard(p.ctx, unverified, txnpool.Unverified); err != nil {
			p.logger.Error().Err(err).
				Msgf("Failed to remove %d unverifiable transactions from the pool", len(unverified))
		}
	}

	return nil
}

func (p *proposer) handleTransactionsFromNeighbors(tx db.RoTx) error {
	state, err := db.ReadCollatorState(tx, p.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	neighborIndexes := common.SliceToMap(state.Neighbors, func(i int, t types.Neighbor) (types.ShardId, int) {
		return t.ShardId, i
	})

	checkLimits := func() bool {
		return p.executionState.GasUsed < p.params.MaxInternalGasInBlock &&
			len(p.proposal.InternalTxns) < p.params.MaxInternalTransactionsInBlock &&
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
		lastBlock, _, err := db.ReadLastBlock(tx, neighborId)
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
			block, err := db.ReadBlockByNumber(tx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outTxnTrie := execution.NewDbTransactionTrieReader(tx, neighborId)
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

						if !checkLimits() {
							break
						}
					}

					p.proposal.InternalTxns = append(p.proposal.InternalTxns, txn)
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

	p.logger.Debug().Msgf("Collected %d incoming transactions from neighbors with %d gas",
		len(p.proposal.InternalTxns), p.executionState.GasUsed)

	p.proposal.CollatorState = state
	return nil
}
