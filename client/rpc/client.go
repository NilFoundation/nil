package rpc

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/assert"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
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
	Eth_getCurrencies                    = "eth_getCurrencies"
	Eth_getShardIdList                   = "eth_getShardIdList"
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

func (c *Client) RawCall(method string, params ...any) (json.RawMessage, error) {
	return c.call(method, params)
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

	bytes := hexutil.FromHex(codeHex)
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

func (c *Client) SendMessage(msg *types.ExternalMessage) (common.Hash, error) {
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

func (c *Client) GetBalance(address types.Address, blockId any) (*types.Uint256, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	balance := types.NewUint256(0)
	if err != nil {
		return balance, err
	}

	params := []any{address.String(), transport.BlockNumberOrHash(blockNrOrHash)}
	res, err := c.call(Eth_getBalance, params)
	if err != nil {
		return balance, err
	}

	err = balance.UnmarshalJSON(res)
	return balance, err
}

func (c *Client) GetCurrencies(address types.Address, blockId any) (map[string]*types.Uint256, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	params := []any{address.String(), transport.BlockNumberOrHash(blockNrOrHash)}
	res, err := c.call(Eth_getCurrencies, params)
	if err != nil {
		return nil, err
	}

	currencies := make(map[string]*types.Uint256)
	err = json.Unmarshal(res, &currencies)
	if err != nil {
		return nil, err
	}

	return currencies, err
}

func (c *Client) GetShardIdList() ([]types.ShardId, error) {
	res, err := c.call(Eth_getShardIdList, []any{})
	if err != nil {
		return []types.ShardId{}, err
	}

	var shardIdList []types.ShardId
	if err := json.Unmarshal(res, &shardIdList); err != nil {
		return []types.ShardId{}, err
	}
	return shardIdList, nil
}

func (c *Client) DeployContract(
	shardId types.ShardId, walletAddress types.Address, bytecode types.Code, value *types.Uint256, pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, bytecode)
	txHash, err := c.sendMessageViaWallet(walletAddress, bytecode, types.NewUint256(100_000), value, []types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *Client) DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload, pk *ecdsa.PrivateKey) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := c.sendExternalMessage(deployPayload.Bytes(), address, pk, true)
	return msgHash, address, err
}

func (c *Client) SendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, gasLimit *types.Uint256, value *types.Uint256,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return c.sendMessageViaWallet(walletAddress, bytecode, gasLimit, value, currencies, contractAddress, pk, false)
}

// RunContract runs bytecode on the specified contract address
func (c *Client) sendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, gasLimit *types.Uint256, value *types.Uint256,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	if value == nil {
		value = types.NewUint256(0)
	}

	totalValue := types.NewUint256(0)
	totalValue.Int.Mul(&gasLimit.Int, execution.GasPrice)
	totalValue.Int.Add(&value.Int, &totalValue.Int)

	intMsg := &types.InternalMessagePayload{
		Data:     bytecode,
		To:       contractAddress,
		Value:    *totalValue,
		GasLimit: *gasLimit,
		Currency: currencies,
		Kind:     kind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}

	walletAbi, err := contracts.GetAbi("Wallet")
	if err != nil {
		return common.EmptyHash, err
	}

	calldataExt, err := walletAbi.Pack("send", intMsgData)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(calldataExt, walletAddress, pk)
}

func (c *Client) SendExternalMessage(
	bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return c.sendExternalMessage(bytecode, contractAddress, pk, false)
}

func (c *Client) sendExternalMessage(
	bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	// Get the sequence number for the wallet
	seqno, err := c.GetTransactionCount(contractAddress, "latest")
	if err != nil {
		return common.EmptyHash, err
	}

	// Create the message with the bytecode to run
	extMsg := &types.ExternalMessage{
		To:    contractAddress,
		Data:  bytecode,
		Seqno: seqno,
		Kind:  kind,
	}

	// Sign the message with the private key
	if pk != nil {
		err = extMsg.Sign(pk)
		if err != nil {
			return common.EmptyHash, err
		}
	}

	// Send the raw transaction
	txHash, err := c.SendMessage(extMsg)
	if err != nil {
		return common.EmptyHash, err
	}
	return txHash, nil
}

func (c *Client) TopUpViaFaucet(contractAddress types.Address, amount *types.Uint256) (common.Hash, error) {
	gasLimit := *types.NewUint256(100_000)
	value := *amount
	value.Add(&value.Int, types.NewUint256(0).Mul(&gasLimit.Int, execution.GasPrice))
	sendMsgInternal := &types.InternalMessagePayload{
		To:       contractAddress,
		Value:    value,
		GasLimit: gasLimit,
		Kind:     types.ExecutionMessageKind,
	}
	sendMsgInternalData, err := sendMsgInternal.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}

	// Make external message to the Faucet
	faucetAbi, err := contracts.GetAbi("Faucet")
	check.PanicIfErr(err)
	calldata, err := faucetAbi.Pack("send", sendMsgInternalData)
	if err != nil {
		return common.EmptyHash, err
	}

	from := types.FaucetAddress
	return c.SendExternalMessage(calldata, from, nil)
}

func (c *Client) Call(args *jsonrpc.CallArgs) (string, error) {
	params := []any{args, "latest"}
	raw, err := c.call("eth_call", params)
	if err != nil {
		return "", err
	}

	var res string
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res, nil
}
