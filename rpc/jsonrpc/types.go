package jsonrpc

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
)

func NewRPCInMessage(message *types.Message, receipt *types.Receipt, index types.MessageIndex, block *types.Block) *RPCInMessage {
	hash := message.Hash()
	if receipt == nil || hash != receipt.MsgHash {
		panic("Msg and receipt are not compatible")
	}

	blockHash := block.Hash()
	chainId := types.NewUint256(0)
	gasUsed := hexutil.Uint64(receipt.GasUsed)
	msgIndex := hexutil.Uint64(index)
	seqno := hexutil.Uint64(message.Seqno)
	result := &RPCInMessage{
		Success:     receipt.Success,
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
		ChainID:     *chainId,
		Signature:   message.Signature,
	}

	return result
}

func NewRPCBlock(
	shardId types.ShardId, block *types.Block, messages []*types.Message, receipts []*types.Receipt, fullTx bool,
) *RPCBlock {
	if block == nil {
		return nil
	}

	messagesRes := make([]any, len(messages))
	if fullTx {
		for i, m := range messages {
			messagesRes[i] = NewRPCInMessage(m, receipts[i], types.MessageIndex(i), block)
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
	}
}

type CallArgs struct {
	From     types.Address   `json:"from"`
	To       types.Address   `json:"to"`
	GasLimit types.Uint256   `json:"gasLimit"`
	GasPrice *types.Uint256  `json:"gasPrice"`
	Value    types.Uint256   `json:"value"`
	Seqno    *hexutil.Uint64 `json:"seqno"`
	Data     hexutil.Bytes   `json:"data"`
	Input    *hexutil.Bytes  `json:"input"`
	ChainID  *hexutil.Big    `json:"chainId"`
}

type RPCInMessage struct {
	Success     bool              `json:"success"`
	BlockHash   *common.Hash      `json:"blockHash"`
	BlockNumber types.BlockNumber `json:"blockNumber"`
	From        types.Address     `json:"from"`
	GasUsed     hexutil.Uint64    `json:"gasUsed"`
	GasPrice    types.Uint256     `json:"gasPrice,omitempty"`
	GasLimit    types.Uint256     `json:"gasLimit,omitempty"`
	Hash        common.Hash       `json:"hash"`
	Seqno       hexutil.Uint64    `json:"seqno"`
	To          *types.Address    `json:"to"`
	Index       *hexutil.Uint64   `json:"index"`
	Value       types.Uint256     `json:"value"`
	ChainID     types.Uint256     `json:"chainId,omitempty"`
	Signature   common.Signature  `json:"signature"`
}

type RPCBlock struct {
	Number         types.BlockNumber `json:"number"`
	Hash           common.Hash       `json:"hash"`
	ParentHash     common.Hash       `json:"parentHash"`
	InMessagesRoot common.Hash       `json:"inMessagesRoot"`
	ReceiptsRoot   common.Hash       `json:"receiptsRoot"`
	ShardId        types.ShardId     `json:"shardId"`
	Messages       []any             `json:"messages"`
}
