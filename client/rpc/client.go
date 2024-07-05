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
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
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
	Eth_gasPrice                         = "eth_gasPrice"
	Eth_chainId                          = "eth_chainId"
)

type Client struct {
	endpoint string
	seqno    uint64
	client   http.Client
	headers  map[string]string
	logger   zerolog.Logger
}

var _ client.Client = (*Client)(nil)

func NewClient(endpoint string, logger zerolog.Logger) *Client {
	return &Client{
		endpoint: endpoint,
		logger:   logger,
	}
}

func NewClientWithDefaultHeaders(endpoint string, logger zerolog.Logger, headers map[string]string) *Client {
	return &Client{
		endpoint: endpoint,
		logger:   logger,
		headers:  headers,
	}
}

func (c *Client) call(method string, params ...any) (json.RawMessage, error) {
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
	c.logger.Trace().RawJSON("request", requestBody).Send()

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToSendRequest, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToReadResponse, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", ErrUnexpectedStatusCode, resp.StatusCode, body)
	}

	var rpcResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		c.logger.Debug().Str("response", string(body)).Msg("failed to unmarshal response")
		return nil, fmt.Errorf("%w: %w", ErrFailedToUnmarshalResponse, err)
	}
	c.logger.Trace().RawJSON("response", body).Send()

	if errorMsg, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("%w: %s", ErrRPCError, errorMsg)
	}

	return rpcResponse["result"], nil
}

func (c *Client) RawCall(method string, params ...any) (json.RawMessage, error) {
	return c.call(method, params...)
}

func (c *Client) GetCode(addr types.Address, blockId any) (types.Code, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Code{}, err
	}

	raw, err := c.call(Eth_getCode, addr, blockNrOrHash)
	if err != nil {
		return types.Code{}, err
	}

	var codeHex string
	if err := json.Unmarshal(raw, &codeHex); err != nil {
		return types.Code{}, err
	}

	return hexutil.FromHex(codeHex), nil
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
	res, err := c.call(Eth_getBlockByHash, shardId, hash, fullTx)
	if err != nil {
		return nil, err
	}
	return toRPCBlock(res)
}

func (c *Client) getBlockByNumber(shardId types.ShardId, num transport.BlockNumber, fullTx bool) (*jsonrpc.RPCBlock, error) {
	res, err := c.call(Eth_getBlockByNumber, shardId, num, fullTx)
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
	res, err := c.call(Eth_sendRawTransaction, hexutil.Bytes(data))
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
	res, err := c.call(Eth_getInMessageByHash, shardId, hash)
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
	res, err := c.call(Eth_getInMessageReceipt, shardId, hash)
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

	res, err := c.call(Eth_getTransactionCount, address, transport.BlockNumberOrHash(blockNrOrHash))
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
	res, err := c.call(Eth_getBlockTransactionCountByNumber, shardId, number)
	if err != nil {
		return 0, err
	}
	return toUint64(res)
}

func (c *Client) getBlockTransactionCountByHash(shardId types.ShardId, hash common.Hash) (uint64, error) {
	res, err := c.call(Eth_getBlockTransactionCountByHash, shardId, hash)
	if err != nil {
		return 0, err
	}
	return toUint64(res)
}

func (c *Client) GetBalance(address types.Address, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}

	res, err := c.call(Eth_getBalance, address.String(), transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return types.Value{}, err
	}

	bigVal := &hexutil.Big{}
	if err := bigVal.UnmarshalJSON(res); err != nil {
		return types.Value{}, err
	}

	return types.NewValueFromBigMust(bigVal.ToInt()), nil
}

func (c *Client) GetCurrencies(address types.Address, blockId any) (types.CurrenciesMap, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	res, err := c.call(Eth_getCurrencies, address.String(), transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return nil, err
	}

	currencies := make(types.RPCCurrenciesMap)
	err = json.Unmarshal(res, &currencies)
	if err != nil {
		return nil, err
	}

	return types.ToCurrenciesMap(currencies), err
}

func (c *Client) GasPrice(shardId types.ShardId) (types.Value, error) {
	res, err := c.call(Eth_gasPrice, shardId)
	if err != nil {
		return types.Value{}, err
	}

	bigVal := &hexutil.Big{}
	if err := bigVal.UnmarshalJSON(res); err != nil {
		return types.Value{}, err
	}

	return types.NewValueFromBigMust(bigVal.ToInt()), nil
}

func (c *Client) ChainId() (types.ChainId, error) {
	res, err := c.call(Eth_chainId)
	if err != nil {
		return types.ChainId(0), err
	}

	val, err := toUint64(res)
	if err != nil {
		return types.ChainId(0), err
	}
	return types.ChainId(val), err
}

func (c *Client) GetShardIdList() ([]types.ShardId, error) {
	res, err := c.call(Eth_getShardIdList)
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
	shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := c.sendMessageViaWallet(walletAddress, payload.Bytes(), 100_000, value, []types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *Client) DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := c.sendExternalMessage(deployPayload.Bytes(), address, nil, true)
	return msgHash, address, err
}

func (c *Client) SendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, gasLimit types.Gas, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return c.sendMessageViaWallet(walletAddress, bytecode, gasLimit, value, currencies, contractAddress, pk, false)
}

// RunContract runs bytecode on the specified contract address
func (c *Client) sendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, gasLimit types.Gas, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	gasPrice, err := c.GasPrice(walletAddress.ShardId())
	if err != nil {
		return common.EmptyHash, err
	}

	intMsg := &types.InternalMessagePayload{
		Data:     bytecode,
		To:       contractAddress,
		Value:    gasLimit.ToValue(gasPrice).Add(value),
		GasLimit: gasLimit,
		Currency: currencies,
		Kind:     kind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}

	calldataExt, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
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

func (c *Client) TopUpViaFaucet(contractAddress types.Address, amount types.Value) (common.Hash, error) {
	callData, err := contracts.NewCallData(contracts.NameFaucet, "withdrawTo", contractAddress, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendExternalMessage(callData, types.FaucetAddress, nil)
}

func (c *Client) Call(args *jsonrpc.CallArgs) (string, error) {
	raw, err := c.call("eth_call", args, "latest")
	if err != nil {
		return "", err
	}

	var res string
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res, nil
}

func (c *Client) CurrencyCreate(contractAddr types.Address, amount types.Value, name string, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "createToken", amount.ToBig(), name, withdraw)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
}

func (c *Client) CurrencyWithdraw(contractAddr types.Address, amount types.Value, toAddr types.Address, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "withdrawToken", amount.ToBig(), toAddr)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
}

func (c *Client) CurrencyMint(contractAddr types.Address, amount types.Value, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "mintToken", amount.ToBig(), withdraw)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
}
