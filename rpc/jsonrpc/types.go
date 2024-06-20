package jsonrpc

import (
	"errors"

	"github.com/NilFoundation/nil/common"
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
	From     types.Address   `json:"from"`
	To       types.Address   `json:"to"`
	GasLimit *types.Uint256  `json:"gasLimit"`
	GasPrice *types.Uint256  `json:"gasPrice"`
	Value    *types.Uint256  `json:"value"`
	Seqno    *hexutil.Uint64 `json:"seqno"`
	Data     hexutil.Bytes   `json:"data"`
	Input    *hexutil.Bytes  `json:"input"`
	ChainId  types.ChainId   `json:"chainId"`
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
// @componentprop To to string true "The address where the message was sent."
// @componentprop Value value string true "The message value."
// @componentprop Currency value array true "Currency values."
type RPCInMessage struct {
	Success     bool                    `json:"success"`
	Data        hexutil.Bytes           `json:"data"`
	BlockHash   *common.Hash            `json:"blockHash"`
	BlockNumber types.BlockNumber       `json:"blockNumber"`
	From        types.Address           `json:"from"`
	GasUsed     hexutil.Uint64          `json:"gasUsed"`
	GasPrice    types.Uint256           `json:"gasPrice,omitempty"`
	GasLimit    types.Uint256           `json:"gasLimit,omitempty"`
	Hash        common.Hash             `json:"hash"`
	Seqno       hexutil.Uint64          `json:"seqno"`
	To          *types.Address          `json:"to"`
	Index       *hexutil.Uint64         `json:"index"`
	Value       types.Uint256           `json:"value"`
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
type RPCReceipt struct {
	Success         bool               `json:"success"`
	GasUsed         uint32             `json:"gasUsed"`
	Bloom           hexutil.Bytes      `json:"bloom"`
	Logs            []*RPCLog          `json:"logs"`
	OutMessages     []common.Hash      `json:"outMessages"`
	OutReceipts     []*RPCReceipt      `json:"outputReceipts"`
	MsgHash         common.Hash        `json:"messageHash"`
	ContractAddress types.Address      `json:"contractAddress"`
	BlockHash       common.Hash        `json:"blockHash,omitempty"`
	BlockNumber     types.BlockNumber  `json:"blockNumber,omitempty"`
	MsgIndex        types.MessageIndex `json:"messageIndex"`
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

	blockHash := block.Hash()
	gasUsed := hexutil.Uint64(receipt.GasUsed)
	msgIndex := hexutil.Uint64(index)
	seqno := hexutil.Uint64(message.Seqno)
	result := &RPCInMessage{
		Success:     receipt.Success,
		Data:        hexutil.Bytes(message.Data),
		BlockHash:   &blockHash,
		BlockNumber: block.Id,
		From:        message.From,
		GasUsed:     gasUsed,
		GasPrice:    message.GasPrice,
		GasLimit:    message.GasLimit,
		Hash:        hash,
		Seqno:       seqno,
		To:          &message.To,
		Index:       &msgIndex,
		Value:       message.Value,
		ChainID:     message.ChainId,
		Signature:   message.Signature,
	}

	return result, nil
}

func NewRPCBlock(
	shardId types.ShardId, block *types.Block, messages []*types.Message, receipts []*types.Receipt, fullTx bool,
) (*RPCBlock, error) {
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
	block *types.Block, index types.MessageIndex, receipt *types.Receipt, outMessages []common.Hash, outReceipts []*RPCReceipt,
) *RPCReceipt {
	if block == nil || receipt == nil {
		return nil
	}

	logs := make([]*RPCLog, len(receipt.Logs))
	for i, log := range receipt.Logs {
		logs[i] = NewRPCLog(log, block.Id)
	}

	return &RPCReceipt{
		Success:         receipt.Success,
		GasUsed:         receipt.GasUsed,
		Bloom:           hexutil.Bytes(receipt.Bloom.Bytes()),
		Logs:            logs,
		OutMessages:     outMessages,
		OutReceipts:     outReceipts,
		MsgHash:         receipt.MsgHash,
		ContractAddress: receipt.ContractAddress,
		BlockHash:       block.Hash(),
		BlockNumber:     block.Id,
		MsgIndex:        index,
	}
}
