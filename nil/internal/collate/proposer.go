package collate

import (
	"context"
	"errors"
	"fmt"
	"math/big"

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
)

func CallGetterByBlock(
	ctx context.Context,
	tx db.RoTx,
	address types.Address,
	block *types.Block,
	calldata []byte,
) ([]byte, error) {
	cfgAccessor, err := config.NewConfigReader(tx, &block.MainShardHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	es, err := execution.NewExecutionState(tx, address.ShardId(), execution.StateParams{
		Block:          block,
		ConfigAccessor: cfgAccessor,
		Mode:           execution.ModeReadOnly,
	})
	if err != nil {
		return nil, err
	}

	extTxn := &types.ExternalTransaction{
		FeePack: types.NewFeePackFromGas(types.DefaultMaxGasInBlock),
		To:      address,
		Data:    calldata,
	}

	txn := extTxn.ToTransaction()

	payer := execution.NewDummyPayer()

	es.AddInTransaction(txn)
	res := es.HandleTransaction(ctx, txn, payer)
	if res.Failed() {
		return nil, fmt.Errorf("transaction failed: %w", res.GetError())
	}
	return res.ReturnData, nil
}

func CallGetterByBlockNumber(
	ctx context.Context,
	tx db.RoTx,
	address types.Address,
	blockNumber types.BlockNumber,
	calldata []byte,
) ([]byte, error) {
	block, err := db.ReadBlockByNumber(tx, address.ShardId(), blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to read block by number %d: %w", blockNumber, err)
	}
	return CallGetterByBlock(ctx, tx, address, block, calldata)
}

func CallGetter(
	ctx context.Context,
	tx db.RoTx,
	address types.Address,
	calldata []byte,
) ([]byte, error) {
	block, _, err := db.ReadLastBlock(tx, address.ShardId())
	if err != nil {
		return nil, fmt.Errorf("failed to read last block: %w", err)
	}
	return CallGetterByBlock(ctx, tx, address, block, calldata)
}

const (
	defaultMaxGasInBlock                 = types.DefaultMaxGasInBlock
	maxTxnsFromPool                      = 10_000
	defaultMaxForwardTransactionsInBlock = 200

	validatorPatchLevel = 1
)

type proposer struct {
	params *Params

	topology ShardTopology
	pool     TxnPool

	logger logging.Logger

	proposal       *execution.ProposalSSZ
	executionState *execution.ExecutionState

	ctx context.Context

	l1BlockFetcher rollup.L1BlockFetcher
}

func newProposer(params *Params, topology ShardTopology, pool TxnPool, logger logging.Logger) *proposer {
	if params.MaxGasInBlock == 0 {
		params.MaxGasInBlock = defaultMaxGasInBlock
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

func (p *proposer) GenerateProposal(ctx context.Context, txFabric db.DB) (*execution.ProposalSSZ, error) {
	p.proposal = &execution.ProposalSSZ{}

	tx, err := txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	prevBlock, prevBlockHash, err := db.ReadLastBlock(tx, p.params.ShardId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch previous block: %w", err)
	}

	if prevBlock.PatchLevel > validatorPatchLevel {
		return nil, fmt.Errorf(
			"block patch level %d is higher than supported %d", prevBlock.PatchLevel, validatorPatchLevel)
	}

	p.setPrevBlockData(prevBlock, prevBlockHash)

	configAccessor, err := config.NewConfigAccessorFromBlockWithTx(tx, prevBlock, p.params.ShardId)
	if err != nil {
		return nil, fmt.Errorf("failed to create config accessor: %w", err)
	}

	p.executionState, err = execution.NewExecutionState(tx, p.params.ShardId, execution.StateParams{
		Block:          prevBlock,
		ConfigAccessor: configAccessor,
		FeeCalculator:  p.params.FeeCalculator,
		Mode:           execution.ModeProposalGen,
	})
	if err != nil {
		return nil, err
	}

	p.logger.Trace().Msg("Collating...")

	if err := p.fetchLastBlockHashes(tx); err != nil {
		return nil, fmt.Errorf("failed to fetch last block hashes: %w", err)
	}

	if err := p.handleL1Attributes(tx, prevBlockHash); err != nil {
		// TODO: change to Error severity once Consensus/Proposer increase time intervals
		p.logger.Trace().Err(err).Msg("Failed to handle L1 attributes")
	}

	if err := p.handleTransactionsFromNeighbors(tx); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from neighbors: %w", err)
	}

	if err := p.handleTransactionsFromPool(); err != nil {
		return nil, fmt.Errorf("failed to handle transactions from pool: %w", err)
	}

	if rollback := p.executionState.GetRollback(); rollback != nil {
		// TODO: verify mainBlockId, actually perform rollback
		p.proposal.PatchLevel = rollback.PatchLevel
		p.proposal.RollbackCounter = rollback.Counter + 1
	}

	if len(
		p.proposal.InternalTxnRefs) == 0 && len(p.proposal.ExternalTxns) == 0 && len(p.proposal.ForwardTxnRefs) == 0 {
		p.logger.Trace().Msg("No transactions collected")
	} else {
		p.logger.Debug().Msgf("Collected %d internal, %d external (%d gas) and %d forward transactions",
			len(p.proposal.InternalTxnRefs),
			len(p.proposal.ExternalTxns),
			p.executionState.GasUsed,
			len(p.proposal.ForwardTxnRefs))
	}

	return p.proposal, nil
}

func (p *proposer) setPrevBlockData(block *types.Block, blockHash common.Hash) {
	p.proposal.PrevBlockId = block.Id
	p.proposal.PrevBlockHash = blockHash
	p.proposal.PatchLevel = block.PatchLevel
	p.proposal.RollbackCounter = block.RollbackCounter
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
		p.proposal.MainShardHash = lastBlockHash
	}

	return nil
}

func (p *proposer) handleL1Attributes(tx db.RoTx, mainShardHash common.Hash) error {
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
	if cfgAccessor, err := config.NewConfigReader(tx, &mainShardHash); err == nil {
		if prevL1Block, err := config.GetParamL1Block(cfgAccessor); err == nil {
			if prevL1Block != nil && prevL1Block.Number >= block.Number.Uint64() {
				return nil
			}
		}
	}

	txId := p.executionState.InTxCounts[types.MainShardId]
	p.executionState.InTxCounts[types.MainShardId] = txId + 1
	txn, err := CreateL1BlockUpdateTransaction(block, txId)
	if err != nil {
		return fmt.Errorf("failed to create L1 block update transaction: %w", err)
	}

	p.logger.Debug().
		Stringer("block_num", block.Number).
		Stringer("base_fee", block.BaseFee).
		Msg("Add L1 block update transaction")

	p.proposal.SpecialTxns = append(p.proposal.SpecialTxns, txn)

	return nil
}

func CreateRollbackCalldata(params *execution.RollbackParams) ([]byte, error) {
	abi, err := contracts.GetAbi(contracts.NameGovernance)
	if err != nil {
		return nil, fmt.Errorf("failed to get Governance ABI: %w", err)
	}
	calldata, err := abi.Pack("rollback",
		params.Version,
		params.Counter,
		params.PatchLevel,
		params.MainBlockId,
		params.ReplayDepth,
		params.SearchDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to pack rollback calldata: %w", err)
	}
	return calldata, nil
}

func CreateL1BlockUpdateTransaction(header *l1types.Header, txId types.TransactionIndex) (*types.Transaction, error) {
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
			Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
			To:      types.L1BlockInfoAddress,
			FeePack: types.NewFeePackFromGas(types.DefaultMaxGasInBlock),
			Data:    calldata,
		},
		TxId: txId,
		From: types.L1BlockInfoAddress,
	}

	return txn, nil
}

func (p *proposer) handleTransaction(txn *types.Transaction, txnHash common.Hash, payer execution.Payer) error {
	if assert.Enable {
		defer func() {
			check.PanicIfNotf(txnHash == txn.Hash(), "Transaction hash changed during execution")
		}()
	}

	p.executionState.AddInTransactionWithHash(txn, txnHash)

	res := p.executionState.HandleTransaction(p.ctx, txn, payer)
	if res.FatalError != nil {
		return res.FatalError
	} else if res.Failed() {
		p.logger.Debug().Stringer(logging.FieldTransactionHash, txnHash).
			Err(res.Error).
			Msg("Transaction execution failed. It doesn't prevent adding it to the block.")
	}

	return nil
}

func (p *proposer) handleTransactionsFromPool() error {
	poolTxns, err := p.pool.Peek(maxTxnsFromPool)
	if err != nil {
		return err
	}

	if len(poolTxns) != 0 {
		p.logger.Debug().Int("txNum", len(poolTxns)).Msg("Start handling transactions from the pool")
	}

	var unverified []common.Hash
	handle := func(mt *types.TxnWithHash) (bool, error) {
		txnHash := mt.Hash()
		txn := mt.Transaction

		if res := execution.ValidateExternalTransaction(p.executionState, txn); res.FatalError != nil {
			return false, res.FatalError
		} else if res.Failed() {
			p.logger.Info().Stringer(logging.FieldTransactionHash, txnHash).
				Err(res.Error).Msg("External txn validation failed. Saved failure receipt. Dropping...")

			execution.AddFailureReceipt(txnHash, txn.To, res)
			unverified = append(unverified, txnHash)
			return false, nil
		}

		acc, err := p.executionState.GetAccount(txn.To)
		if err != nil {
			return false, err
		}

		if err := p.handleTransaction(txn, txnHash, execution.NewAccountPayer(acc, txn)); err != nil {
			return false, err
		}

		return true, nil
	}

	for _, txn := range poolTxns {
		if ok, err := handle(txn); err != nil {
			return err
		} else if ok {
			p.proposal.ExternalTxns = append(p.proposal.ExternalTxns, txn.Transaction)
			if p.executionState.GasUsed > p.params.MaxGasInBlock {
				unverified = append(unverified, txn.Hash())
				break
			}
		}
	}

	if len(unverified) > 0 {
		p.logger.Debug().Msgf("Removing %d unverifiable transactions from the pool", len(unverified))

		if err := p.pool.Discard(p.ctx, unverified, txnpool.Unverified); err != nil {
			p.logger.Error().Err(err).
				Msgf("Failed to remove %d unverifiable transactions from the pool", len(unverified))
		}
	}

	if len(poolTxns) != 0 {
		p.logger.Debug().Int("txAdded", len(p.proposal.ExternalTxns)).Msg("Finish transactions handling")
	}

	return nil
}

func (p *proposer) handleTransactionsFromNeighbors(tx db.RoTx) error {
	// Get our Relayer address
	ourRelayerAddr := types.GetRelayerAddress(p.params.ShardId)

	// Get neighbors from topology
	neighbors := p.topology.GetNeighbors(p.params.ShardId, p.params.NShards, true)

	var parents []*execution.ParentBlock

	checkLimits := func() bool {
		return p.executionState.GasUsed < p.params.MaxGasInBlock &&
			len(p.proposal.ForwardTxnRefs) < p.params.MaxForwardTransactionsInBlock
	}

	// Get inMsgCounts from our Relayer (what we've processed so far)
	inMsgCountsCalldata, err := contracts.NewCallData(contracts.NameRelayer, "getInMsgCount")
	if err != nil {
		return fmt.Errorf("failed to create getInMsgCount calldata: %w", err)
	}

	inMsgCountsData, err := CallGetter(p.ctx, tx, ourRelayerAddr, inMsgCountsCalldata)
	if err != nil {
		return fmt.Errorf("failed to call getInMsgCount: %w", err)
	}

	var inMsgCounts [5]uint64 // Assuming SHARDS_NUM = 5
	abi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	if err := abi.UnpackIntoInterface(&inMsgCounts, "getInMsgCount", inMsgCountsData); err != nil {
		return fmt.Errorf("failed to unpack getInMsgCount result: %w", err)
	}

	// Define Message struct to match the Solidity struct
	type RelayerMessage struct {
		Id       uint64
		From     types.Address
		To       types.Address
		RefundTo types.Address
		BounceTo types.Address
		Value    *big.Int
		Tokens   []struct {
			Token   types.Address
			Balance *big.Int
		}
		ForwardKind       uint8
		FeeCredit         *big.Int
		Data              []byte
		RequestId         *big.Int
		ResponseFeeCredit *big.Int
		IsDeploy          bool
		Salt              *big.Int
	}

	// Get current block numbers from our Relayer
	currBlockNumCalldata, err := contracts.NewCallData(contracts.NameRelayer, "getCurrentBlockNumber")
	if err != nil {
		return fmt.Errorf("failed to create getCurrentBlockNumber calldata: %w", err)
	}

	currBlockNumData, err := CallGetter(p.ctx, tx, ourRelayerAddr, currBlockNumCalldata)
	if err != nil {
		return fmt.Errorf("failed to call getCurrentBlockNumber: %w", err)
	}

	currentBlockNumbers := make([]uint64, p.params.NShards)
	abi, err = contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return fmt.Errorf("failed to get Relayer ABI: %w", err)
	}
	if err := abi.UnpackIntoInterface(&currentBlockNumbers, "getCurrentBlockNumber", currBlockNumData); err != nil {
		return fmt.Errorf("failed to unpack getCurrentBlockNumber result: %w", err)
	}
	if len(currentBlockNumbers) != int(p.params.NShards) {
		return fmt.Errorf("unexpected number of block numbers returned: %d", len(currentBlockNumbers))
	}

	// Track if we need to update block numbers
	needUpdateBlockNumbers := false
	newBlockNumbers := make([]uint64, p.params.NShards) // Copy of currentBlockNumbers
	copy(newBlockNumbers, currentBlockNumbers)

	for _, neighborID := range neighbors {
		if !checkLimits() {
			break
		}

		// Get neighbor's Relayer address
		neighborRelayerAddr := types.GetRelayerAddress(neighborID)

		// Get our current inMsgCount for this neighbor (messages we've processed)
		fromMsgID := inMsgCounts[neighborID]

		// Get the last block number for this neighbor
		lastBlock, _, err := db.ReadLastBlock(tx, neighborID)
		if err != nil {
			p.logger.Warn().Err(err).Msgf("Failed to read last block for shard %d", neighborID)
			continue
		}

		// Start from the current block we're processing for this neighbor
		currentBlockNum := types.BlockNumber(currentBlockNumbers[neighborID])

		// Process blocks up to the last one
		for currentBlockNum <= lastBlock.Id {
			if !checkLimits() {
				break
			}

			// Get the relayer state at this block number
			neighborBlock, err := db.ReadBlockByNumber(tx, neighborID, currentBlockNum)
			if err != nil {
				p.logger.Warn().Err(err).Msgf("Failed to read block %d for shard %d", currentBlockNum, neighborID)
				break
			}

			// Get messages from neighbor Relayer at this block
			messagesProcessed := false
			for checkLimits() {
				// Get up to 50 pending messages at a time for the current block
				getMsgsCalldata, err := contracts.NewCallData(contracts.NameRelayer, "getPendingMessages",
					uint32(p.params.ShardId), fromMsgID, uint32(50))
				if err != nil {
					return fmt.Errorf("failed to create getPendingMessages calldata: %w", err)
				}

				// Use CallGetterByBlockNumber to access the state at this specific block
				msgsData, err := CallGetterByBlockNumber(
					p.ctx,
					tx,
					neighborRelayerAddr,
					currentBlockNum,
					getMsgsCalldata,
				)
				if err != nil {
					p.logger.Warn().Err(err).
						Msgf("Failed to get pending messages for shard %d at block %d", neighborID, currentBlockNum)
					break
				}

				var messages []RelayerMessage
				abi, err := contracts.GetAbi(contracts.NameRelayer)
				if err != nil {
					return fmt.Errorf("failed to get ABI: %w", err)
				}
				if err := abi.UnpackIntoInterface(&messages, "getPendingMessages", msgsData); err != nil {
					return fmt.Errorf("failed to unpack getPendingMessages result: %w", err)
				}

				if len(messages) == 0 {
					// No more messages in this block, break inner loop to move to next block
					break
				}

				messagesProcessed = true

				for _, msg := range messages {
					if !checkLimits() {
						break
					}

					// Convert the Relayer message to a Transaction
					txn := &types.Transaction{
						TransactionDigest: types.TransactionDigest{
							Flags:   types.TransactionFlagsFromKind(true, types.ExecutionTransactionKind),
							FeePack: types.NewFeePackFromFeeCredit(types.NewValueFromBigMust(msg.FeeCredit)),
							To:      msg.To,
							Data:    msg.Data,
						},
						From:      msg.From,
						RefundTo:  msg.RefundTo,
						BounceTo:  msg.BounceTo,
						Value:     types.NewValueFromBigMust(msg.Value),
						RequestId: msg.RequestId.Uint64(),
						TxId:      types.TransactionIndex(msg.Id),
					}

					// Set flags based on message properties
					if msg.IsDeploy {
						txn.Flags.SetBit(types.TransactionFlagDeploy)
					}

					txnHash := txn.Hash()

					// Process the transaction
					if err := p.handleTransaction(
						txn, txnHash, execution.NewTransactionPayer(txn, p.executionState),
					); err != nil {
						return err
					}

					// Find or create parent block reference
					if len(parents) == 0 ||
						parents[len(parents)-1].ShardId != neighborID ||
						parents[len(parents)-1].Block.Id != neighborBlock.Id {
						parents = append(parents, execution.NewParentBlock(neighborID, neighborBlock))
					}

					blockIndex := uint32(len(parents) - 1)
					txIdx := types.TransactionIndex(msg.Id)
					p.proposal.InternalTxnRefs = append(p.proposal.InternalTxnRefs, &execution.InternalTxnReference{
						ParentBlockIndex: blockIndex,
						TxnIndex:         txIdx,
					})

					// Move to the next message
					fromMsgID++
				}
			}

			// If no messages were processed or we hit limits, we're done with this neighbor
			if !messagesProcessed || !checkLimits() {
				break
			}

			// Move to the next block for this neighbor
			currentBlockNum++

			// Update our tracking of which block we've processed for this neighbor
			if uint64(currentBlockNum) > newBlockNumbers[neighborID] {
				newBlockNumbers[neighborID] = uint64(currentBlockNum)
				needUpdateBlockNumbers = true
			}
		}
	}
	// If we need to update block numbers, prepare a transaction
	if needUpdateBlockNumbers {
		// Create updateCurrentBlockNumber calldata
		updateBlockNumCalldata, err := contracts.NewCallData(
			contracts.NameRelayer, "updateCurrentBlockNumber", newBlockNumbers)
		if err != nil {
			return fmt.Errorf("failed to create updateCurrentBlockNumber calldata: %w", err)
		}

		updateTxn := &types.Transaction{
			TransactionDigest: types.TransactionDigest{
				Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
				FeePack: types.NewFeePackFromGas(100_000), // Reasonable gas limit
				To:      ourRelayerAddr,
				Data:    updateBlockNumCalldata,
			},
			From: ourRelayerAddr,
		}

		p.proposal.SpecialTxns = append(p.proposal.SpecialTxns, updateTxn)
	}

	p.logger.Trace().Msgf("Collected %d incoming transactions from neighbors with %d gas",
		len(p.proposal.InternalTxnRefs), p.executionState.GasUsed)

	// Set parent blocks in the proposal
	p.proposal.ParentBlocks = make([]*execution.ParentBlockSSZ, len(parents))
	for i, parent := range parents {
		p.proposal.ParentBlocks[i] = parent.ToSerializable()
	}

	return nil
}
