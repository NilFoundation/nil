package jsonrpc

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/assert"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
)

// @component CallArgs callArgs object "The arguments for the message call."
// @componentprop From from string true "The address from which the message must be called."
// @componentprop GasLimit gasLimit string true "The gas limit for the message."
// @componentprop GasPrice gasPrice string false "The gas price for the message."
// @componentprop Value value integer true "The message value."
// @componentprop Seqno seqno integer true "The sequence number of the message."
// @componentprop Data data string true "The encoded bytecode of the message."
// @componentprop Input input string false "The message input."
// @component propr ChainId chainId integer "The chain id."
type CallArgs struct {
	From     types.Address  `json:"from"`
	To       types.Address  `json:"to"`
	GasLimit types.Gas      `json:"gasLimit"`
	GasPrice types.Value    `json:"gasPrice,omitempty"`
	Value    types.Value    `json:"value"`
	Seqno    hexutil.Uint64 `json:"seqno"`
	Data     hexutil.Bytes  `json:"data"`
	Input    hexutil.Bytes  `json:"input,omitempty"`
	ChainId  types.ChainId  `json:"chainId"`
}

// @component RPCInMessage rpcInMessage object "The message whose information is requested."
// @componentprop BlockHash blockHash string true "The hash of the block containing the message."
// @componentprop BlockNumber blockNumber integer true "The number of the block containin the message."
// @componentprop ChainId chainId integer true "The number of the chain containing the message."
// @componentprop From from string true "The address from where the message was sent."
// @componentprop GasLimit gasLimit string true "The gas limit for the message."
// @componentprop GasPrice gasPrice string true "The gas price paid for the message."
// @componentprop GasUsed gasUsed string true "The amount of gas spent on the message."
// @componentprop Hash hash string true "The message hash."
// @componentprop Index index string true "The message index."
// @componentprop Seqno seqno string true "The sequence number of the message."
// @componentprop Signature signature string true "The message signature."
// @componentprop Success success boolean true "The flag that shows whether the message was successful."
// @componentprop Flags flags array true "The array of message flags."
// @componentprop To to string true "The address where the message was sent."
// @componentprop Value value string true "The message value."
// @componentprop Currency value array true "Currency values."
type RPCInMessage struct {
	Flags       types.MessageFlags      `json:"flags"`
	Success     bool                    `json:"success"`
	Data        hexutil.Bytes           `json:"data"`
	BlockHash   common.Hash             `json:"blockHash"`
	BlockNumber types.BlockNumber       `json:"blockNumber"`
	From        types.Address           `json:"from"`
	GasUsed     types.Gas               `json:"gasUsed"`
	GasPrice    types.Value             `json:"gasPrice,omitempty"`
	GasLimit    types.Gas               `json:"gasLimit,omitempty"`
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
// @componentprop Number number integer true "The block number."
// @componentprop ParentHash parentHash string true "The hash of the parent block."
// @componentprop ReceiptsRoot receiptsRoot string true "The root of the block receipts."
// @componentprop ShardId shardId integer true "The ID of the shard where the block was generated."
type RPCBlock struct {
	Number         types.BlockNumber `json:"number"`
	Hash           common.Hash       `json:"hash"`
	ParentHash     common.Hash       `json:"parentHash"`
	InMessagesRoot common.Hash       `json:"inMessagesRoot"`
	ReceiptsRoot   common.Hash       `json:"receiptsRoot"`
	ShardId        types.ShardId     `json:"shardId"`
	Messages       []any             `json:"messages"`
	ChildBlocks    []common.Hash     `json:"childBlocks"`
	MainChainHash  common.Hash       `json:"mainChainHash"`
	DbTimestamp    uint64            `json:"dbTimestamp"`
}

type HexedDebugRPCBlock struct {
	Content     string                 `json:"content"`
	InMessages  []string               `json:"inMessages"`
	OutMessages []string               `json:"outMessages"`
	Receipts    []string               `json:"receipts"`
	Errors      map[common.Hash]string `json:"errors"`
}

func (b *HexedDebugRPCBlock) EncodeHex(block *types.BlockWithRawExtractedData) error {
	var err error
	b.Content, err = block.ToHexedSSZ()
	if err != nil {
		return err
	}
	b.InMessages = hexutil.EncodeSSZEncodedDataContainer(block.InMessages)
	b.OutMessages = hexutil.EncodeSSZEncodedDataContainer(block.OutMessages)
	b.Receipts = hexutil.EncodeSSZEncodedDataContainer(block.Receipts)
	b.Errors = block.Errors
	return nil
}

func (b *HexedDebugRPCBlock) DecodeHex() (*types.BlockWithRawExtractedData, error) {
	block, err := types.BlockFromHexedSSZ(b.Content)
	if err != nil {
		return nil, err
	}
	inMessages := hexutil.DecodeSSZEncodedDataContainer(b.InMessages)
	outMessages := hexutil.DecodeSSZEncodedDataContainer(b.OutMessages)
	receipts := hexutil.DecodeSSZEncodedDataContainer(b.Receipts)
	return &types.BlockWithRawExtractedData{
		Block:       block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Errors:      b.Errors,
	}, nil
}

func (b *HexedDebugRPCBlock) DecodeHexAndSSZ() (*types.BlockWithExtractedData, error) {
	block, err := b.DecodeHex()
	if err != nil {
		return nil, err
	}
	return block.DecodeSSZ()
}

func EncodeBlockWithRawExtractedData(block *types.BlockWithRawExtractedData) (*HexedDebugRPCBlock, error) {
	b := new(HexedDebugRPCBlock)
	if err := b.EncodeHex(block); err != nil {
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
	GasUsed         types.Gas          `json:"gasUsed"`
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

func NewRPCInMessage(message *types.Message, receipt *types.Receipt, index types.MessageIndex, block *types.Block) (*RPCInMessage, error) {
	hash := message.Hash()
	if receipt == nil || hash != receipt.MsgHash {
		return nil, errors.New("msg and receipt are not compatible")
	}

	result := &RPCInMessage{
		Flags:       message.Flags,
		Success:     receipt.Success,
		Data:        hexutil.Bytes(message.Data),
		BlockHash:   block.Hash(),
		BlockNumber: block.Id,
		From:        message.From,
		GasUsed:     receipt.GasUsed,
		GasPrice:    message.GasPrice,
		GasLimit:    message.GasLimit,
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

	messagesRes := make([]any, len(messages))
	var err error
	if fullTx {
		for i, m := range messages {
			messagesRes[i], err = NewRPCInMessage(m, receipts[i], types.MessageIndex(i), block)
			if err != nil {
				return nil, err
			}
		}
	} else {
		for i, m := range messages {
			messagesRes[i] = m.Hash()
		}
	}

	return &RPCBlock{
		Number:         block.Id,
		Hash:           block.Hash(),
		ParentHash:     block.PrevBlock,
		InMessagesRoot: block.InMessagesRoot,
		ReceiptsRoot:   block.ReceiptsRoot,
		ShardId:        shardId,
		Messages:       messagesRes,
		ChildBlocks:    childBlocks,
		MainChainHash:  block.MainChainHash,
		DbTimestamp:    dbTimestamp,
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

func NewRPCReceipt(
	shardId types.ShardId, block *types.Block, index types.MessageIndex, receipt *types.Receipt,
	outMessages []common.Hash, outReceipts []*RPCReceipt, temporary bool, errorMessage string,
) *RPCReceipt {
	if receipt == nil {
		return nil
	}

	var blockNumber types.BlockNumber
	var blockHash common.Hash
	if block != nil {
		blockNumber = block.Id
		blockHash = block.Hash()
	}
	logs := make([]*RPCLog, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs[i] = NewRPCLog(log, blockNumber)
	}

	res := &RPCReceipt{
		Success:         receipt.Success,
		Status:          receipt.Status.String(),
		GasUsed:         receipt.GasUsed,
		Logs:            logs,
		OutMessages:     outMessages,
		OutReceipts:     outReceipts,
		MsgHash:         receipt.MsgHash,
		ContractAddress: receipt.ContractAddress,
		BlockHash:       blockHash,
		BlockNumber:     blockNumber,
		MsgIndex:        index,
		ShardId:         shardId,
		Temporary:       temporary,
		ErrorMessage:    errorMessage,
	}

	// Set only non-empty bloom
	if len(receipt.Logs) > 0 {
		res.Bloom = receipt.Bloom.Bytes()
	} else if assert.Enable {
		for _, b := range receipt.Bloom {
			check.PanicIfNotf(b == 0, "bloom must be zero for empty logs")
		}
	}

	return res
}
