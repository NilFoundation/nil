package rpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
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
	Eth_call                             = "eth_call"
	Eth_estimateFee                      = "eth_estimateFee"
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
	Debug_getBlockByHash                 = "debug_getBlockByHash"
	Debug_getBlockByNumber               = "debug_getBlockByNumber"
)

const (
	Db_initDbTimestamp = "db_initDbTimestamp"
	Db_get             = "db_get"
	Db_exists          = "db_exists"
	Db_existsInShard   = "db_existsInShard"
	Db_getFromShard    = "db_getFromShard"
)

type Client struct {
	endpoint string
	seqno    atomic.Uint64
	client   http.Client
	headers  map[string]string
	logger   zerolog.Logger
}

type Request struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	Id      uint64 `json:"id"`
}

func NewRequest(id uint64, method string, params []any) *Request {
	return &Request{
		Version: "2.0",
		Method:  method,
		Id:      id,
		Params:  params,
	}
}

var (
	_ client.Client       = (*Client)(nil)
	_ client.BatchRequest = (*BatchRequestImpl)(nil)
)

type BatchRequestImpl struct {
	requests []*Request
	client   *Client
}

func (b *BatchRequestImpl) getBlock(shardId types.ShardId, blockId any, fullTx bool, isDebug bool) (uint64, error) {
	id := len(b.requests)

	r, err := b.client.getBlockRequest(shardId, blockId, fullTx, isDebug)
	if err != nil {
		return 0, err
	}

	b.requests = append(b.requests, r)
	return uint64(id), nil
}

func (b *BatchRequestImpl) GetBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error) {
	return b.getBlock(shardId, blockId, fullTx, false)
}

func (b *BatchRequestImpl) GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error) {
	return b.getBlock(shardId, blockId, fullTx, true)
}

func NewClient(endpoint string, logger zerolog.Logger) *Client {
	return NewClientWithDefaultHeaders(endpoint, logger, nil)
}

func NewClientWithDefaultHeaders(endpoint string, logger zerolog.Logger, headers map[string]string) *Client {
	c := &Client{
		endpoint: endpoint,
		logger:   logger,
		headers:  headers,
	}

	if strings.HasPrefix(endpoint, "unix://") {
		socketPath := strings.TrimPrefix(endpoint, "unix://")
		if socketPath == "" {
			return nil
		}
		c.endpoint = "http://unix"
		c.client = http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}
	} else if strings.HasPrefix(endpoint, "tcp://") {
		endpoint := "http://" + strings.TrimPrefix(endpoint, "tcp://")
		c.endpoint = endpoint
	}

	return c
}

func (c *Client) getNextId() uint64 {
	return c.seqno.Add(1)
}

func (c *Client) newRequest(method string, params ...any) *Request {
	return NewRequest(c.getNextId(), method, params)
}

func (c *Client) call(method string, params ...any) (json.RawMessage, error) {
	request := c.newRequest(method, params...)
	return c.performRequest(request)
}

func (c *Client) performRequest(request *Request) (json.RawMessage, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	body, err := c.PlainTextCall(requestBody)
	if err != nil {
		return nil, err
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

func (c *Client) performRequests(requests []*Request) ([]json.RawMessage, error) {
	requestsBody, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	body, err := c.PlainTextCall(requestsBody)
	if err != nil {
		return nil, err
	}

	var rpcResponse []map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		c.logger.Debug().Str("response", string(body)).Msg("failed to unmarshal response")
		return nil, fmt.Errorf("%w: %w", ErrFailedToUnmarshalResponse, err)
	}
	c.logger.Trace().RawJSON("response", body).Send()

	results := make([]json.RawMessage, len(rpcResponse))
	for i, resp := range rpcResponse {
		if errorMsg, ok := resp["error"]; ok {
			return nil, fmt.Errorf("%w: %s (%d)", ErrRPCError, errorMsg, i)
		}
		results[i] = resp["result"]
	}
	return results, nil
}

func (c *Client) PlainTextCall(requestBody []byte) (json.RawMessage, error) {
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
	return body, nil
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

func (c *Client) getBlockRequest(shardId types.ShardId, blockId any, fullTx bool, isDebug bool) (*Request, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	var request *Request
	if blockNrOrHash.BlockHash != nil {
		m := Eth_getBlockByHash
		if isDebug {
			m = Debug_getBlockByHash
		}
		request = c.newRequest(m, shardId, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		m := Eth_getBlockByNumber
		if isDebug {
			m = Debug_getBlockByNumber
		}
		request = c.newRequest(m, shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	check.PanicIfNot(request != nil)
	return request, nil
}

func (c *Client) GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
	request, err := c.getBlockRequest(shardId, blockId, fullTx, false)
	if err != nil {
		return nil, err
	}

	res, err := c.performRequest(request)
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

func (c *Client) GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.HexedDebugRPCBlock, error) {
	request, err := c.getBlockRequest(shardId, blockId, fullTx, true)
	if err != nil {
		return nil, err
	}

	res, err := c.performRequest(request)
	if err != nil {
		return nil, err
	}

	return toRawBlock(res)
}

func toRawBlock(raw json.RawMessage) (*jsonrpc.HexedDebugRPCBlock, error) {
	var blockInfo *jsonrpc.HexedDebugRPCBlock
	if err := json.Unmarshal(raw, &blockInfo); err != nil {
		return nil, err
	}
	return blockInfo, nil
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
	txHash, err := client.SendMessageViaWallet(c, walletAddress, payload.Bytes(), types.GasToValue(100_000), types.GasToValue(100_000),
		value, []types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *Client) DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := client.SendExternalMessage(c, deployPayload.Bytes(), address, nil, feeCredit, true, false)
	return msgHash, address, err
}

func (c *Client) SendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, externalFeeCredit, internalFeeCredit, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return client.SendMessageViaWallet(c, walletAddress, bytecode, externalFeeCredit, internalFeeCredit, value, currencies, contractAddress, pk, false)
}

func (c *Client) SendExternalMessage(
	bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, feeCredit types.Value,
) (common.Hash, error) {
	return client.SendExternalMessage(c, bytecode, contractAddress, pk, feeCredit, false, false)
}

func (c *Client) TopUpViaFaucet(contractAddress types.Address, amount types.Value) (common.Hash, error) {
	callData, err := contracts.NewCallData(contracts.NameFaucet, "withdrawTo", contractAddress, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	return client.SendExternalMessage(c, callData, types.FaucetAddress, nil, types.GasToValue(100_000), false, true)
}

func (c *Client) Call(args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	raw, err := c.call(Eth_call, args, blockNrOrHash, stateOverride)
	if err != nil {
		return nil, err
	}

	var res *jsonrpc.CallRes
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) EstimateFee(args *jsonrpc.CallArgs, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}

	raw, err := c.call(Eth_estimateFee, args, blockNrOrHash)
	if err != nil {
		return types.Value{}, err
	}

	var res types.Value
	if err := json.Unmarshal(raw, &res); err != nil {
		return types.Value{}, err
	}
	return res, nil
}

func (c *Client) SetCurrencyName(contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "setCurrencyName", name)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk, types.GasToValue(100_000))
}

func (c *Client) ChangeCurrencyAmount(contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey, mint bool) (common.Hash, error) {
	method := "mintCurrency"
	if !mint {
		method = "burnCurrency"
	}
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, method, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk, types.GasToValue(100_000))
}

func callDbAPI[T any](c *Client, method string, params ...any) (T, error) {
	var res T
	raw, err := c.call(method, params...)
	if err != nil {
		if strings.Contains(err.Error(), jsonrpc.ErrApiKeyNotFound.Error()) {
			return res, db.ErrKeyNotFound
		}
		return res, err
	}

	return res, json.Unmarshal(raw, &res)
}

func (c *Client) DbInitTimestamp(ts uint64) error {
	_, err := c.call(Db_initDbTimestamp, ts)
	return err
}

func (c *Client) DbGet(tableName db.TableName, key []byte) ([]byte, error) {
	return callDbAPI[[]byte](c, Db_get, tableName, key)
}

func (c *Client) DbGetFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	return callDbAPI[[]byte](c, Db_getFromShard, shardId, tableName, key)
}

func (c *Client) DbExists(tableName db.TableName, key []byte) (bool, error) {
	return callDbAPI[bool](c, Db_exists, tableName, key)
}

func (c *Client) DbExistsInShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	return callDbAPI[bool](c, Db_existsInShard, shardId, tableName, key)
}

func (c *Client) CreateBatchRequest() client.BatchRequest {
	return &BatchRequestImpl{
		requests: make([]*Request, 0),
		client:   c,
	}
}

func (c *Client) BatchCall(req client.BatchRequest) ([]any, error) {
	r, ok := req.(*BatchRequestImpl)
	check.PanicIfNot(ok)

	responses, err := c.performRequests(r.requests)
	if err != nil {
		return nil, err
	}

	result := make([]any, len(responses))
	for i, resp := range responses {
		method := r.requests[i].Method
		switch method {
		case Eth_getBlockByHash, Eth_getBlockByNumber:
			block, err := toRPCBlock(resp)
			if err != nil {
				return nil, err
			}
			result[i] = block
		case Debug_getBlockByHash, Debug_getBlockByNumber:
			block, err := toRawBlock(resp)
			if err != nil {
				return nil, err
			}
			result[i] = block
		default:
			result[i] = resp
		}
	}

	return result, nil
}
