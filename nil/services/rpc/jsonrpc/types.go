package jsonrpc

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type (
	Contract       = rpctypes.Contract
	CallArgs       = rpctypes.CallArgs
	StateOverrides = rpctypes.StateOverrides
)

// @component RPCInMessage rpcInMessage object "The message whose information is requested."
// @componentprop BlockHash blockHash string true "The hash of the block containing the message."
// @componentprop BlockNumber blockNumber integer true "The number of the block containin the message."
// @componentprop ChainId chainId integer true "The number of the chain containing the message."
// @componentprop From from string true "The address from where the message was sent."
// @componentprop FeeCredit feeCredit string true "The fee credit for the message."
// @componentprop GasUsed gasUsed string true "The amount of gas spent on the message."
// @componentprop Hash hash string true "The message hash."
// @componentprop Index index string true "The message index."
// @componentprop Seqno seqno string true "The sequence number of the message."
// @componentprop Signature signature string true "The message signature."
// @componentprop Success success boolean true "The flag that shows whether the message was successful."
// @componentprop Flags flags string true "The array of message flags."
// @componentprop To to string true "The address where the message was sent."
// @componentprop Value value string true "The message value."
// @componentprop Currency value array true "Currency values."
type RPCInMessage struct {
	Flags       types.MessageFlags      `json:"flags"`
	Success     bool                    `json:"success"`
	RequestId   uint64                  `json:"requestId"`
	Data        hexutil.Bytes           `json:"data"`
	BlockHash   common.Hash             `json:"blockHash"`
	BlockNumber types.BlockNumber       `json:"blockNumber"`
	From        types.Address           `json:"from"`
	GasUsed     types.Gas               `json:"gasUsed"`
	FeeCredit   types.Value             `json:"feeCredit,omitempty"`
	Hash        common.Hash             `json:"hash"`
	Seqno       hexutil.Uint64          `json:"seqno"`
	To          types.Address           `json:"to"`
	RefundTo    types.Address           `json:"refundTo"`
	BounceTo    types.Address           `json:"bounceTo"`
	Index       hexutil.Uint64          `json:"index"`
	Value       types.Value             `json:"value"`
	Currency    []types.CurrencyBalance `json:"currency,omitempty"`
	ChainID     types.ChainId           `json:"chainId,omitempty"`
	Signature   types.Signature         `json:"signature"`
}

// @component RPCBlock rpcBlock object "The block whose information was requested."
// @componentprop Hash hash string true "The hash of the block."
// @componentprop Messages messages array true "The messages included in the block."
// @componentprop MessageHashes string array true "The hashes of messages included in the block."
// @componentprop Number number integer true "The block number."
// @componentprop ParentHash parentHash string true "The hash of the parent block."
// @componentprop ReceiptsRoot receiptsRoot string true "The root of the block receipts."
// @componentprop ShardId shardId integer true "The ID of the shard where the block was generated."
type RPCBlock struct {
	Number              types.BlockNumber `json:"number"`
	Hash                common.Hash       `json:"hash"`
	ParentHash          common.Hash       `json:"parentHash"`
	InMessagesRoot      common.Hash       `json:"inMessagesRoot"`
	ReceiptsRoot        common.Hash       `json:"receiptsRoot"`
	ChildBlocksRootHash common.Hash       `json:"childBlocksRootHash"`
	ShardId             types.ShardId     `json:"shardId"`
	Messages            []*RPCInMessage   `json:"messages,omitempty"`
	MessageHashes       []common.Hash     `json:"messageHashes,omitempty"`
	ChildBlocks         []common.Hash     `json:"childBlocks"`
	MainChainHash       common.Hash       `json:"mainChainHash"`
	DbTimestamp         uint64            `json:"dbTimestamp"`
	GasPrice            types.Value       `json:"gasPrice"`
	LogsBloom           hexutil.Bytes     `json:"logsBloom,omitempty"`
}

type DebugRPCBlock struct {
	Content     hexutil.Bytes          `json:"content"`
	ChildBlocks []common.Hash          `json:"childBlocks"`
	InMessages  []hexutil.Bytes        `json:"inMessages"`
	OutMessages []hexutil.Bytes        `json:"outMessages"`
	Receipts    []hexutil.Bytes        `json:"receipts"`
	Errors      map[common.Hash]string `json:"errors"`
}

func (b *DebugRPCBlock) Encode(block *types.RawBlockWithExtractedData) error {
	b.Content = block.Block
	b.ChildBlocks = block.ChildBlocks
	b.InMessages = hexutil.FromBytesSlice(block.InMessages)
	b.OutMessages = hexutil.FromBytesSlice(block.OutMessages)
	b.Receipts = hexutil.FromBytesSlice(block.Receipts)
	b.Errors = block.Errors
	return nil
}

func (b *DebugRPCBlock) Decode() (*types.RawBlockWithExtractedData, error) {
	return &types.RawBlockWithExtractedData{
		Block:       b.Content,
		ChildBlocks: b.ChildBlocks,
		InMessages:  hexutil.ToBytesSlice(b.InMessages),
		OutMessages: hexutil.ToBytesSlice(b.OutMessages),
		Receipts:    hexutil.ToBytesSlice(b.Receipts),
		Errors:      b.Errors,
	}, nil
}

func (b *DebugRPCBlock) DecodeSSZ() (*types.BlockWithExtractedData, error) {
	block, err := b.Decode()
	if err != nil {
		return nil, err
	}
	return block.DecodeSSZ()
}

func EncodeRawBlockWithExtractedData(block *types.RawBlockWithExtractedData) (*DebugRPCBlock, error) {
	b := &DebugRPCBlock{}
	if err := b.Encode(block); err != nil {
		return nil, err
	}
	return b, nil
}

// @component RPCReceipt rpcReceipt object "The receipt whose structure is requested."
// @componentprop BlockHash blockHash string true "The hash of the block containing the message whose receipt is requested."
// @componentprop BlockNumber blockNumber integer true "The number of the block containin the message whose receipt is requested."
// @componentprop Bloom bloom string true "The receipt bloom filter."
// @componentprop ContractAddress contractAddress string true "The address of the contract that has originated the message whose receipt is requested."
// @componentprop GasUsed gasUsed string true "The amount of gas spent on the message whose receipt is requested."
// @componentprop GasPrice gasPrice string true "The gas price at the time of processing the message."
// @componentprop Logs logs array true "The logs attached to the receipt."
// @componentprop MessageHash messageHash string true "The hash of the message whose receipt is requested."
// @componentprop MessageIndex messageIndex integer true "The index of the message whose receipt is requested."
// @componentprop OutMsgIndex outMsgIndex integer true "The index of the outgoing message whose receipt is requested."
// @componentprop OutMsgNum outMsgNum integer true "The number of the outgoing messages whose receipt is requested."
// @componentprop OutReceipts outputReceipts array true "Receipts of the outgoing messages. Set to nil for messages that have not yet been processed."
// @componentprop Success success boolean true "The flag that shows whether the message was successful."
// @componentprop Status status string false "Status shows concrete error of the executed message."
// @componentprop Temporary temporary boolean false "The flag that shows whether the message is temporary."
// @componentprop ErrorMessage errorMessage string false "The error in case the message processing was unsuccessful."
type RPCReceipt struct {
	Success         bool               `json:"success"`
	Status          string             `json:"status"`
	FailedPc        uint               `json:"failedPc"`
	IncludedInMain  bool               `json:"includedInMain"`
	GasUsed         types.Gas          `json:"gasUsed"`
	Forwarded       types.Value        `json:"forwarded"`
	GasPrice        types.Value        `json:"gasPrice"`
	Bloom           hexutil.Bytes      `json:"bloom,omitempty"`
	Logs            []*RPCLog          `json:"logs"`
	OutMessages     []common.Hash      `json:"outMessages"`
	OutReceipts     []*RPCReceipt      `json:"outputReceipts"`
	MsgHash         common.Hash        `json:"messageHash"`
	ContractAddress types.Address      `json:"contractAddress"`
	BlockHash       common.Hash        `json:"blockHash"`
	BlockNumber     types.BlockNumber  `json:"blockNumber"`
	MsgIndex        types.MessageIndex `json:"messageIndex"`
	ShardId         types.ShardId      `json:"shardId"`
	Temporary       bool               `json:"temporary,omitempty"`
	ErrorMessage    string             `json:"errorMessage,omitempty"`
}

type RPCLog struct {
	Address     types.Address     `json:"address"`
	Topics      []common.Hash     `json:"topics"`
	Data        hexutil.Bytes     `json:"data"`
	BlockNumber types.BlockNumber `json:"blockNumber"`
}

func (re *RPCReceipt) IsComplete() bool {
	if re == nil || len(re.OutReceipts) != len(re.OutMessages) {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.IsComplete() {
			return false
		}
	}
	return true
}

func (re *RPCReceipt) AllSuccess() bool {
	if !re.Success {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.AllSuccess() {
			return false
		}
	}
	return true
}

// IsCommitted returns true if the receipt is complete and its block is included in the main chain.
func (re *RPCReceipt) IsCommitted() bool {
	if re == nil || len(re.OutReceipts) != len(re.OutMessages) {
		return false
	}
	if !re.IncludedInMain {
		return false
	}
	for _, receipt := range re.OutReceipts {
		if !receipt.IsCommitted() {
			return false
		}
	}
	return true
}

func NewRPCInMessage(
	message *types.Message, receipt *types.Receipt, index types.MessageIndex,
	blockHash common.Hash, blockId types.BlockNumber,
) (*RPCInMessage, error) {
	hash := message.Hash()
	if receipt == nil || hash != receipt.MsgHash {
		return nil, errors.New("msg and receipt are not compatible")
	}

	result := &RPCInMessage{
		Flags:       message.Flags,
		Success:     receipt.Success,
		RequestId:   message.RequestId,
		Data:        hexutil.Bytes(message.Data),
		BlockHash:   blockHash,
		BlockNumber: blockId,
		From:        message.From,
		GasUsed:     receipt.GasUsed,
		FeeCredit:   message.FeeCredit,
		Hash:        hash,
		Seqno:       hexutil.Uint64(message.Seqno),
		To:          message.To,
		RefundTo:    message.RefundTo,
		BounceTo:    message.BounceTo,
		Index:       hexutil.Uint64(index),
		Value:       message.Value,
		ChainID:     message.ChainId,
		Signature:   message.Signature,
	}

	return result, nil
}

func NewRPCBlock(shardId types.ShardId, data *BlockWithEntities, fullTx bool) (*RPCBlock, error) {
	block := data.Block
	messages := data.InMessages
	receipts := data.Receipts
	childBlocks := data.ChildBlocks
	dbTimestamp := data.DbTimestamp

	if block == nil {
		return nil, nil
	}

	messagesRes := make([]*RPCInMessage, 0, len(messages))
	messageHashesRes := make([]common.Hash, 0, len(messages))
	blockHash := block.Hash(shardId)
	blockId := block.Id
	if fullTx {
		for i, m := range messages {
			msg, err := NewRPCInMessage(m, receipts[i], types.MessageIndex(i), blockHash, blockId)
			if err != nil {
				return nil, err
			}
			messagesRes = append(messagesRes, msg)
		}
	} else {
		for _, m := range messages {
			messageHashesRes = append(messageHashesRes, m.Hash())
		}
	}

	// Set only non-empty bloom
	var bloom hexutil.Bytes
	for _, b := range block.LogsBloom {
		if b != 0 {
			bloom = block.LogsBloom.Bytes()
			break
		}
	}

	return &RPCBlock{
		Number:              blockId,
		Hash:                blockHash,
		ParentHash:          block.PrevBlock,
		InMessagesRoot:      block.InMessagesRoot,
		ReceiptsRoot:        block.ReceiptsRoot,
		ChildBlocksRootHash: block.ChildBlocksRootHash,
		ShardId:             shardId,
		Messages:            messagesRes,
		MessageHashes:       messageHashesRes,
		ChildBlocks:         childBlocks,
		MainChainHash:       block.MainChainHash,
		DbTimestamp:         dbTimestamp,
		GasPrice:            block.GasPrice,
		LogsBloom:           bloom,
	}, nil
}

func NewRPCLog(
	log *types.Log, blockId types.BlockNumber,
) *RPCLog {
	if log == nil {
		return nil
	}

	return &RPCLog{
		Address:     log.Address,
		Topics:      log.Topics,
		Data:        log.Data,
		BlockNumber: blockId,
	}
}

func NewRPCReceipt(info *rawapitypes.ReceiptInfo) (*RPCReceipt, error) {
	if info == nil {
		return nil, nil
	}

	receipt := &types.Receipt{}
	if err := receipt.UnmarshalSSZ(info.ReceiptSSZ); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}

	logs := make([]*RPCLog, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs[i] = NewRPCLog(log, info.BlockId)
	}

	outReceipts := make([]*RPCReceipt, len(info.OutReceipts))
	for i, outReceipt := range info.OutReceipts {
		var err error
		outReceipts[i], err = NewRPCReceipt(outReceipt)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %d out receipt: %w", i, err)
		}
	}

	res := &RPCReceipt{
		Success:         receipt.Success,
		Status:          receipt.Status.String(),
		FailedPc:        uint(receipt.FailedPc),
		GasUsed:         receipt.GasUsed,
		Forwarded:       receipt.Forwarded,
		GasPrice:        info.GasPrice,
		Logs:            logs,
		OutMessages:     info.OutMessages,
		OutReceipts:     outReceipts,
		MsgHash:         receipt.MsgHash,
		ContractAddress: receipt.ContractAddress,
		BlockHash:       info.BlockHash,
		BlockNumber:     info.BlockId,
		MsgIndex:        info.Index,
		ShardId:         info.ShardId,
		Temporary:       info.Temporary,
		ErrorMessage:    info.ErrorMessage,
		IncludedInMain:  info.IncludedInMain,
	}

	// Set only non-empty bloom
	if len(receipt.Logs) > 0 {
		res.Bloom = receipt.Bloom.Bytes()
	} else if assert.Enable {
		for _, b := range receipt.Bloom {
			check.PanicIfNotf(b == 0, "bloom must be zero for empty logs")
		}
	}

	return res, nil
}

// @component DebugRPCContract debugRpcContract object "The debug contract whose structure is requested."
// @componentprop Code HEX-encoded contract code
// @componentprop Contract serialized types.SmartContract structure
// @componentprop Proof serialized data for MPT access operation proving
// @componentprop Storage storage slice of key-value pairs of the data in storage
type DebugRPCContract struct {
	// path, node type, next ref, branches, data
	Code     hexutil.Bytes                 `json:"code"`
	Contract hexutil.Bytes                 `json:"contract"`
	Proof    hexutil.Bytes                 `json:"proof"`
	Storage  map[common.Hash]types.Uint256 `json:"storage"`
}

// @component OutMessage outMessage object "Outbound message produced by eth_call and result of its execution."
// @componentprop Message message object true "Message data"
// @componentprop Data data string false "Result of VM execution."
// @componentprop CoinsUsed coinsUsed string true "The amount of coins spent on the message."
// @componentprop OutMessages outMessages array false "Outbound messages produced by eth_call and result of its execution."
// @componentprop Error error string false "Error produced by the message."
type OutMessage struct {
	Message     *types.OutboundMessage `json:"message"`
	Data        hexutil.Bytes          `json:"data,omitempty"`
	CoinsUsed   types.Value            `json:"coinsUsed"`
	OutMessages []*OutMessage          `json:"outMessages,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

func toOutMessages(input []*rpctypes.OutMessage) ([]*OutMessage, error) {
	if len(input) == 0 {
		return nil, nil
	}

	output := make([]*OutMessage, len(input))
	for i, msg := range input {
		outMsgs, err := toOutMessages(msg.OutMessages)
		if err != nil {
			return nil, err
		}

		decoded := &types.OutboundMessage{
			Message:     &types.Message{},
			ForwardKind: msg.ForwardKind,
		}
		if err := decoded.UnmarshalSSZ(msg.MessageSSZ); err != nil {
			return nil, err
		}

		output[i] = &OutMessage{
			Message:     decoded,
			Data:        msg.Data,
			CoinsUsed:   msg.CoinsUsed,
			OutMessages: outMsgs,
			Error:       msg.Error,
		}
	}
	return output, nil
}

// @component CallRes callRes object "Response for eth_call."
// @componentprop Data data string false "Result of VM execution."
// @componentprop CoinsUsed coinsUsed string true "The amount of coins spent on the message."
// @componentprop OutMessages outMessages array false "Outbound messages produced by the message."
// @componentprop Error error string false "Error produced during the call."
// @componentprop StateOverrides stateOverrides object false "Updated contracts state."
type CallRes struct {
	Data           hexutil.Bytes  `json:"data,omitempty"`
	CoinsUsed      types.Value    `json:"coinsUsed"`
	OutMessages    []*OutMessage  `json:"outMessages,omitempty"`
	Error          string         `json:"error,omitempty"`
	StateOverrides StateOverrides `json:"stateOverrides,omitempty"`
}

func toCallRes(input *rpctypes.CallResWithGasPrice) (*CallRes, error) {
	var err error
	output := &CallRes{}
	output.Data = input.Data
	output.CoinsUsed = input.CoinsUsed
	output.Error = input.Error
	output.StateOverrides = input.StateOverrides
	output.OutMessages, err = toOutMessages(input.OutMessages)
	return output, err
}
