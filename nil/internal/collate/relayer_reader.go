package collate

import (
	"context"
	"fmt"
	"iter"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
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

// RelayerMessage represents a message in the Relayer contract
type RelayerMessage struct {
	Id       uint64
	Seqno    uint64
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
	RequestId         uint64
	ResponseFeeCredit *big.Int
	IsDeploy          bool
	IsRefund          bool
	Salt              *big.Int
}

// ToTransaction converts a RelayerMessage to a Transaction
func (msg *RelayerMessage) ToTransaction() *types.Transaction {
	flags := types.NewTransactionFlags(types.TransactionFlagInternal)
	if msg.IsRefund {
		flags.SetBit(types.TransactionFlagRefund)
	}

	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   flags,
			FeePack: types.NewFeePackFromFeeCredit(types.NewValueFromBigMust(msg.FeeCredit)),
			To:      msg.To,
			Seqno:   types.Seqno(msg.Seqno),
			Data:    msg.Data,
		},
		From:      msg.From,
		RefundTo:  msg.RefundTo,
		BounceTo:  msg.BounceTo,
		Value:     types.NewValueFromBigMust(msg.Value),
		RequestId: msg.RequestId,
		TxId:      types.TransactionIndex(msg.Id),
	}

	// We don't set deploy flag because deploy transaction is processed by receiveTxDeploy in Relayer
	// if msg.IsDeploy {
	// 	txn.Flags.SetBit(types.TransactionFlagDeploy)
	// }

	return txn
}

// TransactionWithParent couples a transaction with information about its parent block
type TransactionWithParent struct {
	Transaction     *types.Transaction
	Hash            common.Hash
	ParentIndex     int
	ParentBlock     *types.Block
	NeighborShardId types.ShardId
}

// RelayerReader provides read-only access to Relayer contract methods
type RelayerReader struct {
	relayerAddress types.Address
	blockNumber    types.BlockNumber
}

// NewRelayerReader creates a new RelayerReader for the given shard ID
func NewRelayerReader(shardId types.ShardId, blockNumber types.BlockNumber) *RelayerReader {
	return &RelayerReader{
		relayerAddress: types.GetRelayerAddress(shardId),
		blockNumber:    blockNumber,
	}
}

// GetInMsgCounts retrieves the inMsgCounts array from the Relayer contract
func (r *RelayerReader) GetInMsgCounts(ctx context.Context, tx db.RoTx) ([]uint64, error) {
	inMsgCounts := make([]uint64, 0)

	calldata, err := contracts.NewCallData(contracts.NameRelayer, "getInMsgCount")
	if err != nil {
		return inMsgCounts, fmt.Errorf("failed to create getInMsgCount calldata: %w", err)
	}

	data, err := CallGetterByBlockNumber(ctx, tx, r.relayerAddress, r.blockNumber, calldata)
	if err != nil {
		return inMsgCounts, fmt.Errorf("failed to call getInMsgCount: %w", err)
	}

	relayerAbi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return inMsgCounts, fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	if err := relayerAbi.UnpackIntoInterface(&inMsgCounts, "getInMsgCount", data); err != nil {
		return inMsgCounts, fmt.Errorf("failed to unpack getInMsgCount result: %w", err)
	}

	return inMsgCounts, nil
}

// GetCurrentBlockNumbers retrieves the currentBlockNumber array from the Relayer contract
func (r *RelayerReader) GetCurrentBlockNumbers(ctx context.Context, tx db.RoTx) ([]uint64, error) {
	blockNumbers := make([]uint64, 0)

	calldata, err := contracts.NewCallData(contracts.NameRelayer, "getCurrentBlockNumber")
	if err != nil {
		return blockNumbers, fmt.Errorf("failed to create getCurrentBlockNumber calldata: %w", err)
	}

	data, err := CallGetterByBlockNumber(ctx, tx, r.relayerAddress, r.blockNumber, calldata)
	if err != nil {
		return blockNumbers, fmt.Errorf("failed to call getCurrentBlockNumber: %w", err)
	}

	relayerAbi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return blockNumbers, fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	if err := relayerAbi.UnpackIntoInterface(&blockNumbers, "getCurrentBlockNumber", data); err != nil {
		return blockNumbers, fmt.Errorf("failed to unpack getCurrentBlockNumber result: %w", err)
	}

	return blockNumbers, nil
}

// GetPendingMessages retrieves pending messages from the Relayer contract at a specific block
func (r *RelayerReader) GetPendingMessages(
	ctx context.Context,
	tx db.RoTx,
	targetShardId types.ShardId,
	fromMsgId uint64,
	batchSize uint32,
) ([]RelayerMessage, error) {
	calldata, err := contracts.NewCallData(contracts.NameRelayer, "getPendingMessages",
		uint32(targetShardId), fromMsgId, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create getPendingMessages calldata: %w", err)
	}

	data, err := CallGetterByBlockNumber(ctx, tx, r.relayerAddress, r.blockNumber, calldata)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages at block %d: %w", r.blockNumber, err)
	}

	var messages []RelayerMessage
	relayerAbi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return nil, fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	if err := relayerAbi.UnpackIntoInterface(&messages, "getPendingMessages", data); err != nil {
		return nil, fmt.Errorf("failed to unpack getPendingMessages result: %w", err)
	}

	return messages, nil
}

// GetMessageById retrieves a specific message by ID from the Relayer contract
func (r *RelayerReader) GetMessageById(
	ctx context.Context,
	tx db.RoTx,
	shardId types.ShardId,
	msgId uint64,
) (RelayerMessage, error) {
	calldata, err := contracts.NewCallData(contracts.NameRelayer, "getMessageById", shardId, msgId)
	if err != nil {
		return RelayerMessage{}, fmt.Errorf("failed to create getMessageById calldata: %w", err)
	}

	data, err := CallGetterByBlockNumber(ctx, tx, r.relayerAddress, r.blockNumber, calldata)
	if err != nil {
		return RelayerMessage{}, fmt.Errorf("failed to get message by id: %w", err)
	}

	relayerAbi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return RelayerMessage{}, fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	var msg RelayerMessage
	if err := relayerAbi.UnpackIntoInterface(&msg, "getMessageById", data); err != nil {
		return RelayerMessage{}, fmt.Errorf("failed to unpack getMessageById result: %w", err)
	}

	return msg, nil
}

// RelayerMessageQueueReader encapsulates logic for reading messages from Relayer contracts
type RelayerMessageQueueReader struct {
	ctx        context.Context
	tx         db.RoTx
	nShards    uint32
	ourShardId types.ShardId
	ourRelayer *RelayerReader
	neighbors  []types.ShardId
	logger     logging.Logger

	inMsgCounts         []uint64
	currentBlockNumbers []uint64
	newBlockNumbers     []uint64

	parents []*execution.ParentBlock

	needUpdateBlockNumbers bool
	hitLimit               bool
}

// NewRelayerMessageQueueReader creates a new RelayerReader
func NewRelayerMessageQueueReader(
	ctx context.Context,
	tx db.RoTx,
	nShards uint32,
	shardId types.ShardId,
	neighbors []types.ShardId,
	logger logging.Logger,
) (*RelayerMessageQueueReader, error) {
	block, _, err := db.ReadLastBlock(tx, shardId)
	if err != nil {
		return nil, fmt.Errorf("failed to read last block: %w", err)
	}
	r := &RelayerMessageQueueReader{
		ctx:        ctx,
		tx:         tx,
		nShards:    nShards,
		ourShardId: shardId,
		ourRelayer: NewRelayerReader(shardId, block.Id),
		neighbors:  neighbors,
		logger:     logger,
		parents:    make([]*execution.ParentBlock, 0),
	}

	// Initialize with current values from the relayer contract
	r.inMsgCounts, err = r.ourRelayer.GetInMsgCounts(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get inMsgCounts: %w", err)
	}

	r.currentBlockNumbers, err = r.ourRelayer.GetCurrentBlockNumbers(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get currentBlockNumbers: %w", err)
	}

	// Start with current block numbers, will update as needed
	r.newBlockNumbers = make([]uint64, r.nShards)
	copy(r.newBlockNumbers, r.currentBlockNumbers)

	return r, nil
}

// findOrAddParentBlock finds a parent block in the list or adds it if not found
func (r *RelayerMessageQueueReader) findOrAddParentBlock(
	neighborId types.ShardId,
	neighborBlock *types.Block,
) (int, error) {
	for i, parent := range r.parents {
		if parent.ShardId == neighborId && parent.Block.Id == neighborBlock.Id {
			return i, nil
		}
	}

	// Not found, add it
	r.parents = append(r.parents, execution.NewParentBlock(neighborId, neighborBlock))
	return len(r.parents) - 1, nil
}

// markBlockProcessed updates the tracking of which blocks have been processed
func (r *RelayerMessageQueueReader) markBlockProcessed(neighborId types.ShardId, blockNumber types.BlockNumber) {
	if uint64(blockNumber) > r.newBlockNumbers[neighborId] {
		r.newBlockNumbers[neighborId] = uint64(blockNumber)
		r.needUpdateBlockNumbers = true
	}
}

// GetParentBlocks returns the parent blocks that were referenced
func (r *RelayerMessageQueueReader) GetParentBlocks() []*execution.ParentBlock {
	return r.parents
}

// GenerateUpdateBlockNumbersTransaction creates a transaction to update the blockNumbers in the contract
func (r *RelayerMessageQueueReader) GenerateUpdateBlockNumbersTransaction() (*types.Transaction, bool) {
	if !r.needUpdateBlockNumbers {
		return nil, false
	}

	calldata, err := contracts.NewCallData(contracts.NameRelayer, "updateCurrentBlockNumber", r.newBlockNumbers)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to create updateCurrentBlockNumber calldata")
		return nil, false
	}

	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
			FeePack: types.NewFeePackFromGas(100_000), // Reasonable gas limit
			To:      r.ourRelayer.relayerAddress,
			Data:    calldata,
		},
		From:     r.ourRelayer.relayerAddress,
		RefundTo: r.ourRelayer.relayerAddress,
		BounceTo: r.ourRelayer.relayerAddress,
	}

	return txn, true
}

// HitLimit returns whether the reader hit a resource limit
func (r *RelayerMessageQueueReader) HitLimit() bool {
	return r.hitLimit
}

// SetLimit marks that the reader has hit a resource limit
func (r *RelayerMessageQueueReader) SetLimit() {
	r.hitLimit = true
}

// Iter returns an iterator over transactions from neighboring shards
func (r *RelayerMessageQueueReader) Iter(checkLimits func() bool) iter.Seq[*TransactionWithParent] {
	return func(yield func(*TransactionWithParent) bool) {
		// Process each neighbor
		for _, neighborId := range r.neighbors {
			// Stop if we've hit resource limits
			if !checkLimits() {
				r.hitLimit = true
				return
			}

			// Get our current inMsgCount for this neighbor
			fromMsgId := r.inMsgCounts[neighborId]

			// Get the last block for this neighbor
			lastBlock, _, err := db.ReadLastBlock(r.tx, neighborId)
			if err != nil {
				r.logger.Warn().Err(err).Msgf("Failed to read last block for shard %d", neighborId)
				continue
			}

			// Start from the current block we're processing for this neighbor
			currentBlockNum := types.BlockNumber(r.currentBlockNumbers[neighborId])

			// Process blocks up to the last one
			for currentBlockNum <= lastBlock.Id {
				// Stop if we hit resource limits
				if !checkLimits() {
					r.hitLimit = true
					return
				}

				// Read the block at this position
				neighborBlock, err := db.ReadBlockByNumber(r.tx, neighborId, currentBlockNum)
				if err != nil {
					r.logger.Warn().Err(err).Msgf("Failed to read block %d for shard %d", currentBlockNum, neighborId)
					break
				}

				// Track if we processed any messages in this block
				// messagesProcessed := false

				// Process messages in batches for this block
				const batchSize = 50

				// Loop until we process all messages in this block or hit limits
				for {
					// Stop if we hit resource limits
					if !checkLimits() {
						r.hitLimit = true
						return
					}

					// Create a relayer reader for this neighbor
					neighbourRelayer := NewRelayerReader(neighborId, currentBlockNum)

					// Get a batch of pending messages for this block
					messages, err := neighbourRelayer.GetPendingMessages(
						r.ctx,
						r.tx,
						r.ourShardId,
						fromMsgId,
						batchSize,
					)
					if err != nil {
						r.logger.Warn().Err(err).Msgf(
							"Failed to get pending messages from shard %d at block %d",
							neighborId,
							currentBlockNum,
						)
						break
					}

					// If no more messages in this block, move to the next block
					if len(messages) == 0 {
						break
					}

					// Mark that we found messages in this block
					// messagesProcessed = true

					// Process each message in the batch
					for _, msg := range messages {
						// Check limits before processing each message
						if !checkLimits() {
							r.hitLimit = true
							return
						}

						// Convert message to transaction
						txn := msg.ToTransaction()
						txnHash := txn.Hash()

						// Find or add the parent block reference
						parentIdx, err := r.findOrAddParentBlock(neighborId, neighborBlock)
						if err != nil {
							r.logger.Error().Err(err).Msg("Failed to add parent block")
							continue
						}

						if neighborId == 0 && neighborBlock.Id == 1 {
							r.logger.Debug().Msgf("Relayer message: %s", txnHash)
						}

						// Create the transaction with parent info and yield it
						txWithParent := &TransactionWithParent{
							Transaction:     txn,
							Hash:            txnHash,
							ParentIndex:     parentIdx,
							ParentBlock:     neighborBlock,
							NeighborShardId: neighborId,
						}

						// If consumer returns false, stop iteration
						if !yield(txWithParent) {
							return
						}

						// Move to the next message ID
						fromMsgId++
					}
				}
				// Move to the next block
				currentBlockNum++
				// Update our tracking of which block we've processed
				r.markBlockProcessed(neighborId, currentBlockNum)

				// If we processed any messages, update the block number tracking
				// if messagesProcessed {
				// 	// Move to the next block
				// 	currentBlockNum++
				// 	// Update our tracking of which block we've processed
				// 	r.markBlockProcessed(neighborId, currentBlockNum)
				// } else {
				// 	// No messages in this block, still move to the next one
				// 	currentBlockNum++
				// }
			}
		}
	}
}
