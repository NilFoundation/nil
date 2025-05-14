package execution

import (
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

// MessageType constants match the ones in the MessageQueue contract
const (
	MessageTypePending   = 0
	MessageTypeDelivered = 1
	MessageTypeFailed    = 2
	MessageTypePruned    = 3

	MessageTypeRegular  = 0
	MessageTypeResponse = 1
	MessageTypeBounce   = 2
)

// Message represents a message in the MessageQueue
type Message struct {
	ID               [32]byte             `json:"id"`
	MessageType      uint8                `json:"messageType"`
	Status           uint8                `json:"status"`
	SourceChain      *big.Int             `json:"sourceChain"`
	DestinationChain *big.Int             `json:"destinationChain"`
	From             types.Address        `json:"from"`
	To               types.Address        `json:"to"`
	RefundTo         types.Address        `json:"refundTo"`
	BounceTo         types.Address        `json:"bounceTo"`
	Value            *big.Int             `json:"value"`
	ForwardKind      uint8                `json:"forwardKind"`
	FeeCredit        *big.Int             `json:"feeCredit"`
	Tokens           []types.TokenBalance `json:"tokens"`
	Data             []byte               `json:"data"`
	RequestID        *big.Int             `json:"requestId"`
	ResponseGas      *big.Int             `json:"responseGas"`
	Timestamp        *big.Int             `json:"timestamp"`
	Nonce            *big.Int             `json:"nonce"`
	BlockNumber      *big.Int             `json:"blockNumber"`
}

// GetPendingMessages retrieves pending messages from a message queue
func GetPendingMessages(es *ExecutionState, targetShard types.ShardId, maxCount int) ([]Message, error) {
	logger := es.logger.With().Str("function", "GetPendingMessages").Logger()

	mqAddress := types.GetMessageQueueAddress(es.ShardId)
	account, err := es.GetAccount(mqAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get message queue account: %w", err)
	}

	if account == nil {
		logger.Debug().Msg("MessageQueue contract not deployed yet")
		return nil, nil
	}

	// Get the ABI for MessageQueue
	mqAbi, err := contracts.GetAbi(contracts.NameMessageQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
	}

	// Pack the function call data
	calldata, err := mqAbi.Pack("getPendingMessages", big.NewInt(int64(targetShard)), big.NewInt(int64(maxCount)))
	if err != nil {
		return nil, fmt.Errorf("failed to pack getPendingMessages call: %w", err)
	}

	// Create a VM to execute the static call
	if err := es.newVm(true, mqAddress); err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}
	defer es.resetVm()

	// Make the static call to retrieve pending messages
	ret, leftOver, err := es.evm.StaticCall(
		(vm.AccountRef)(mqAddress),
		mqAddress,
		calldata,
		types.DefaultMaxGasInBlock.Uint64(),
	)
	if err != nil {
		return nil, fmt.Errorf("StaticCall to MessageQueue failed: %w", err)
	}

	logger.Debug().Uint64("gas_left", leftOver).Msg("MessageQueue.getPendingMessages gas used")

	// Unpack the returned messages
	var messages []Message
	if err := mqAbi.UnpackIntoInterface(&messages, "getPendingMessages", ret); err != nil {
		return nil, fmt.Errorf("failed to unpack messages: %w", err)
	}

	logger.Debug().Int("count", len(messages)).Msg("Retrieved pending messages")
	return messages, nil
}

// CreateMessageDeliveryTransaction creates a transaction to deliver a cross-shard message
func CreateMessageDeliveryTransaction(
	message Message,
	fromShard types.ShardId,
	toShard types.ShardId,
) (*types.Transaction, error) {
	// Get relayer address
	relayerAddr := types.GetRelayerAddress(toShard)

	// Get the ABI for Relayer
	relayerAbi, err := contracts.GetAbi(contracts.NameRelayer)
	if err != nil {
		return nil, fmt.Errorf("failed to get Relayer ABI: %w", err)
	}

	// Determine which function to call based on message type
	var methodName string
	var args []interface{}

	switch message.MessageType {
	case MessageTypeRegular:
		methodName = "executeMessage"
		args = []interface{}{
			message.ID,
			message.SourceChain,
			message.From,
			message.To,
			message.BounceTo,
			message.Value,
			message.Tokens,
			message.Data,
			message.RequestID,
			message.ResponseGas,
		}
	case MessageTypeResponse:
		// Decode response data which contains success, responseData, requestId, originalMessageId
		responseAbi, _ := abi.NewType("tuple(bool,bytes,uint256,bytes32)", "", nil)
		responseArgs := abi.Arguments{abi.Argument{Type: responseAbi}}
		values, err := responseArgs.Unpack(message.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to unpack response data: %w", err)
		}

		methodName = "executeResponse"
		args = []interface{}{
			message.ID,
			message.From,
			message.To,
			message.Value,
			values[0], // success
			values[1], // responseData
			values[2], // requestId
			message.FeeCredit,
		}
	case MessageTypeBounce:
		methodName = "executeBounce"
		args = []interface{}{
			message.ID,
			message.To,
			message.Value,
			message.Tokens,
			message.Data,
		}
	default:
		return nil, fmt.Errorf("unknown message type: %d", message.MessageType)
	}

	// Pack the calldata for the relayer function
	calldata, err := relayerAbi.Pack(methodName, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack %s calldata: %w", methodName, err)
	}

	// Create transaction to send to Relayer
	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
			To:      relayerAddr,
			FeePack: types.NewFeePackFromFeeCredit(types.NewValueFromBigMust(message.FeeCredit)),
			Data:    calldata,
		},
		Value: types.NewValueFromBigMust(message.Value),
		TxId:  0, // Will be set by the caller
		From:  types.GetMessageQueueAddress(fromShard),
	}

	return txn, nil
}

// CreateMQPruneTransaction creates a transaction to prune processed messages
func CreateMQPruneTransaction(
	shardId types.ShardId,
) (*types.Transaction, error) {
	// Get the MessageQueue address
	mqAddress := types.GetMessageQueueAddress(shardId)

	// Get the ABI for MessageQueue
	mqAbi, err := contracts.GetAbi(contracts.NameMessageQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
	}

	// Pack pruning call for this destination
	calldata, err := mqAbi.Pack("pruneAll")
	if err != nil {
		return nil, fmt.Errorf("failed to pack prune call: %w", err)
	}

	// Create transaction to MessageQueue
	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:   types.NewTransactionFlags(types.TransactionFlagInternal),
			To:      mqAddress,
			FeePack: types.NewFeePackFromGas(100000), // Lower gas for pruning
			Data:    calldata,
		},
		Value: types.NewZeroValue(),
		TxId:  0, // Will be set by the caller
		From:  types.GetMessageQueueAddress(shardId),
	}

	return txn, nil
}

// GetMessageQueueStats gets queue statistics for debugging/monitoring
// func GetMessageQueueStats(es *ExecutionState, destShard types.ShardId) (total, pending, firstIndex, lastPruned uint64, err error) {
// 	mqAddress := GetMessageQueueAddress(es.ShardId)
// 	account, err := es.GetAccount(mqAddress)
// 	if err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("failed to get message queue account: %w", err)
// 	}

// 	if account == nil {
// 		return 0, 0, 0, 0, nil
// 	}

// 	// Get the ABI for MessageQueue
// 	mqAbi, err := contracts.GetAbi(contracts.NameMessageQueue)
// 	if err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
// 	}

// 	// Pack the function call data
// 	calldata, err := mqAbi.Pack("getQueueStats", big.NewInt(int64(destShard)))
// 	if err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("failed to pack getQueueStats call: %w", err)
// 	}

// 	// Create a VM to execute the static call
// 	if err := es.newVm(true, mqAddress); err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("failed to create VM: %w", err)
// 	}
// 	defer es.resetVm()

// 	// Make the static call to retrieve queue stats
// 	ret, _, err := es.evm.StaticCall(
// 		(vm.AccountRef)(mqAddress),
// 		mqAddress,
// 		calldata,
// 		100000, // Lower gas for read-only operation
// 	)
// 	if err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("StaticCall to MessageQueue failed: %w", err)
// 	}

// 	// Unpack the returned stats
// 	var totalBig, pendingBig, firstIndexBig, lastPrunedBig *big.Int
// 	if err := mqAbi.Unpack(&[]interface{}{&totalBig, &pendingBig, &firstIndexBig, &lastPrunedBig}, "getQueueStats", ret); err != nil {
// 		return 0, 0, 0, 0, fmt.Errorf("failed to unpack queue stats: %w", err)
// 	}

// 	return totalBig.Uint64(), pendingBig.Uint64(), firstIndexBig.Uint64(), lastPrunedBig.Uint64(), nil
// }
