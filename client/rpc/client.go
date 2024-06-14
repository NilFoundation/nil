package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/assert"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
)

var (
	ErrFailedToMarshalRequest    = errors.New("failed to marshal request")
	ErrFailedToSendRequest       = errors.New("failed to send request")
	ErrUnexpectedStatusCode      = errors.New("unexpected status code")
	ErrFailedToReadResponse      = errors.New("failed to read response")
	ErrFailedToUnmarshalResponse = errors.New("failed to unmarshal response")
	ErrRPCError                  = errors.New("rpc error")
)

const (
	Eth_getCode                          = "eth_getCode"
	Eth_getBlockByHash                   = "eth_getBlockByHash"
	Eth_getBlockByNumber                 = "eth_getBlockByNumber"
	Eth_sendRawTransaction               = "eth_sendRawTransaction"
	Eth_getInMessageByHash               = "eth_getInMessageByHash"
	Eth_getInMessageReceipt              = "eth_getInMessageReceipt"
	Eth_getTransactionCount              = "eth_getTransactionCount"
	Eth_getBlockTransactionCountByNumber = "eth_getBlockTransactionCountByNumber"
	Eth_getBlockTransactionCountByHash   = "eth_getBlockTransactionCountByHash"
	Eth_getBalance                       = "eth_getBalance"
)

type Client struct {
	endpoint string
	seqno    uint64
	client   http.Client
}

var _ client.Client = (*Client)(nil)

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
	}
}

func (c *Client) call(method string, params []any) (json.RawMessage, error) {
	c.seqno++

	request := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      c.seqno,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToSendRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToReadResponse, err)
	}

	var rpcResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToUnmarshalResponse, err)
	}

	if errorMsg, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("%w: %s", ErrRPCError, errorMsg)
	}

	return rpcResponse["result"], nil
}

func (c *Client) Call(method string, params ...any) (map[string]any, error) {
	raw, err := c.call(method, params)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetCode(addr types.Address, blockId any) (types.Code, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Code{}, err
	}

	params := []any{addr, blockNrOrHash}
	raw, err := c.call(Eth_getCode, params)
	if err != nil {
		return types.Code{}, err
	}

	var codeHex string
	if err := json.Unmarshal(raw, &codeHex); err != nil {
		return types.Code{}, err
	}

	bytes, err := hex.DecodeString(codeHex)
	if err != nil {
		return types.Code{}, err
	}

	return types.Code(bytes), nil
}

func (c *Client) GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.getBlockByHash(shardId, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.getBlockByNumber(shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	if assert.Enable {
		panic("Unreachable")
	}

	return nil, nil
}

func (c *Client) getBlockByHash(shardId types.ShardId, hash common.Hash, fullTx bool) (*jsonrpc.RPCBlock, error) {
	params := []any{shardId, hash, fullTx}
	res, err := c.call(Eth_getBlockByHash, params)
	if err != nil {
		return nil, err
	}
	return toRPCBlock(res)
}

func (c *Client) getBlockByNumber(shardId types.ShardId, num transport.BlockNumber, fullTx bool) (*jsonrpc.RPCBlock, error) {
	params := []any{shardId, num, fullTx}
	res, err := c.call(Eth_getBlockByNumber, params)
	if err != nil {
		return nil, err
	}
	return toRPCBlock(res)
}

func toRPCBlock(raw json.RawMessage) (*jsonrpc.RPCBlock, error) {
	var block *jsonrpc.RPCBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, err
	}
	return block, nil
}

func (c *Client) SendMessage(msg *types.Message) (common.Hash, error) {
	data, err := msg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendRawTransaction(data)
}

func (c *Client) SendRawTransaction(data []byte) (common.Hash, error) {
	params := []any{hexutil.Bytes(data)}
	res, err := c.call(Eth_sendRawTransaction, params)
	if err != nil {
		return common.EmptyHash, err
	}

	var hash common.Hash
	if err := json.Unmarshal(res, &hash); err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}

func (c *Client) GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error) {
	params := []any{shardId, hash}
	res, err := c.call(Eth_getInMessageByHash, params)
	if err != nil {
		return nil, err
	}

	var msg *jsonrpc.RPCInMessage
	if err := json.Unmarshal(res, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Client) GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	params := []any{shardId, hash}
	res, err := c.call(Eth_getInMessageReceipt, params)
	if err != nil {
		return nil, err
	}

	var receipt *jsonrpc.RPCReceipt
	if err := json.Unmarshal(res, &receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (c *Client) GetTransactionCount(address types.Address, blockId any) (types.Seqno, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	params := []any{address, transport.BlockNumberOrHash(blockNrOrHash)}
	res, err := c.call(Eth_getTransactionCount, params)
	if err != nil {
		return 0, err
	}

	val, err := toUint64(res)
	if err != nil {
		return 0, err
	}
	return types.Seqno(val), nil
}

func toUint64(raw json.RawMessage) (uint64, error) {
	input := strings.TrimSpace(string(raw))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}
	return strconv.ParseUint(input, 0, 64)
}

func (c *Client) GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.getBlockTransactionCountByHash(shardId, *blockNrOrHash.BlockHash)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.getBlockTransactionCountByNumber(shardId, *blockNrOrHash.BlockNumber)
	}
	if assert.Enable {
		panic("Unreachable")
	}
	return 0, nil
}

func (c *Client) getBlockTransactionCountByNumber(shardId types.ShardId, number transport.BlockNumber) (uint64, error) {
	params := []any{shardId, number}
	res, err := c.call(Eth_getBlockTransactionCountByNumber, params)
	if err != nil {
		return 0, err
	}
	return toUint64(res)
}

func (c *Client) getBlockTransactionCountByHash(shardId types.ShardId, hash common.Hash) (uint64, error) {
	params := []any{shardId, hash}
	res, err := c.call(Eth_getBlockTransactionCountByHash, params)
	if err != nil {
		return 0, err
	}
	return toUint64(res)
}

func (c *Client) GetBalance(address types.Address, blockId any) (*big.Int, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return big.NewInt(0), err
	}

	params := []any{address.String(), transport.BlockNumberOrHash(blockNrOrHash)}
	res, err := c.call(Eth_getBalance, params)
	if err != nil {
		return big.NewInt(0), err
	}

	balance := hexutil.Big{}
	err = balance.UnmarshalJSON(res)
	return balance.ToInt(), err
}
