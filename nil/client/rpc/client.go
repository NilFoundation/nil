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
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// CallError represents an error that occurs during a remote procedure call,
// including the data marshalling and unmarshalling parts
type CallError struct {
	message string
}

func (e CallError) Error() string {
	return e.message
}

func newCallErr(message string) error {
	return &CallError{message}
}

var (
	ErrFailedToMarshalRequest    = newCallErr("failed to marshal request")
	ErrFailedToSendRequest       = newCallErr("failed to send request")
	ErrUnexpectedStatusCode      = newCallErr("unexpected status code")
	ErrFailedToReadResponse      = newCallErr("failed to read response")
	ErrFailedToUnmarshalResponse = newCallErr("failed to unmarshal response")
	ErrRPCError                  = newCallErr("rpc error")
	/*
		This error means that your code exceeds the maximum supported size.
		Try compiling your contract with the usage of solc --optimize flag,
		providing small values to --optimize-runs.
		For more information go to
		https://ethereum.org/en/developers/tutorials/downsizing-contracts-to-fight-the-contract-size-limit/`
	*/
	ErrTxnDataTooLong = newCallErr("data is too long")
)

const (
	Eth_call                             = "eth_call"
	Eth_estimateFee                      = "eth_estimateFee"
	Eth_getCode                          = "eth_getCode"
	Eth_getBlockByHash                   = "eth_getBlockByHash"
	Eth_getBlockByNumber                 = "eth_getBlockByNumber"
	Eth_sendRawTransaction               = "eth_sendRawTransaction"
	Eth_getInTransactionByHash           = "eth_getInTransactionByHash"
	Eth_getInTransactionReceipt          = "eth_getInTransactionReceipt"
	Eth_getTransactionCount              = "eth_getTransactionCount"
	Eth_getBlockTransactionCountByNumber = "eth_getBlockTransactionCountByNumber"
	Eth_getBlockTransactionCountByHash   = "eth_getBlockTransactionCountByHash"
	Eth_getBalance                       = "eth_getBalance"
	Eth_getTokens                        = "eth_getTokens" //nolint:gosec
	Eth_getShardIdList                   = "eth_getShardIdList"
	Eth_getNumShards                     = "eth_getNumShards"
	Eth_gasPrice                         = "eth_gasPrice"
	Eth_chainId                          = "eth_chainId"
	Debug_getBlockByHash                 = "debug_getBlockByHash"
	Debug_getBlockByNumber               = "debug_getBlockByNumber"
	Debug_getContract                    = "debug_getContract"
	Web3_clientVersion                   = "web3_clientVersion"
	Dev_doPanicOnShard                   = "dev_doPanicOnShard"
	Txpool_getTxpoolStatus               = "txpool_getTxpoolStatus"
	Txpool_getTxpoolContent              = "txpool_getTxpoolContent"
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
	logger   logging.Logger
	retrier  *common.RetryRunner
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

func (b *BatchRequestImpl) SendTransactionViaSmartContract(
	ctx context.Context, smartAccountAddress types.Address, bytecode types.Code, fee types.FeePack, value types.Value,
	tokens []types.TokenBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (uint64, error) {
	id := len(b.requests)

	r, err := b.client.getSendTransactionViaSmartContractRequest(
		ctx, smartAccountAddress, bytecode, fee, value, tokens, contractAddress, pk, false, id)
	if err != nil {
		return 0, err
	}

	b.requests = append(b.requests, r)
	return uint64(id), nil
}

type CallParam struct {
	Bytecode []byte
	Address  types.Address
	Count    int
}

// RunContractBatch runs bytecodes on the specified contract addresses as one batch
func RunContractBatch(ctx context.Context, client *Client, smartAccount types.Address, callParams []CallParam,
	fee types.FeePack, value types.Value, tokens []types.TokenBalance, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	batch := client.CreateBatchRequest()

	for _, p := range callParams {
		for range p.Count {
			_, err := batch.SendTransactionViaSmartContract(ctx, smartAccount, p.Bytecode, fee, value, tokens,
				p.Address, pk)
			if err != nil {
				return common.EmptyHash, err
			}
		}
	}

	resp, err := client.BatchCall(ctx, batch)
	if err != nil {
		return common.EmptyHash, err
	}

	// get hash of the latest message
	rawTxn, ok := resp[len(resp)-1].(json.RawMessage)
	if !ok {
		return common.EmptyHash, errors.New("result is not bytes")
	}

	var txHash common.Hash
	if err = json.Unmarshal(rawTxn, &txHash); err != nil {
		return common.EmptyHash, err
	}
	return txHash, nil
}

func NewClient(endpoint string, logger logging.Logger, opts ...Option) *Client {
	return NewClientWithDefaultHeaders(endpoint, logger, nil, opts...)
}

func NewRawClient(endpoint string, logger logging.Logger, opts ...Option) client.RawClient {
	return NewClient(endpoint, logger, opts...)
}

func NewHttpClient(url string) (http.Client, string) {
	client := http.Client{}
	endpoint := url
	if strings.HasPrefix(url, "unix://") {
		socketPath := strings.TrimPrefix(url, "unix://")
		endpoint = "http://unix"
		check.PanicIfNot(socketPath != "")
		client.Transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		}
	} else if strings.HasPrefix(url, "tcp://") {
		endpoint = "http://" + strings.TrimPrefix(url, "tcp://")
	}
	return client, endpoint
}

func NewClientWithDefaultHeaders(
	url string, logger logging.Logger, headers map[string]string, opts ...Option,
) *Client {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	client, endpoint := NewHttpClient(url)
	c := &Client{
		endpoint: endpoint,
		logger:   logger,
		headers:  headers,
		client:   client,
	}

	if cfg.retry != nil {
		retrier := common.NewRetryRunner(*cfg.retry, c.logger)
		c.retrier = &retrier
	}

	return c
}

func (c *Client) getNextId() uint64 {
	return c.seqno.Add(1)
}

func (c *Client) newRequest(method string, params ...any) *Request {
	return NewRequest(c.getNextId(), method, params)
}

func (c *Client) call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	request := c.newRequest(method, params...)
	return c.performRequest(ctx, request)
}

func (c *Client) performRequest(ctx context.Context, request *Request) (json.RawMessage, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	body, err := c.PlainTextCall(ctx, requestBody)
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

func (c *Client) performRequests(ctx context.Context, requests []*Request) ([]json.RawMessage, error) {
	requestsBody, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	var results []json.RawMessage

	call := func(ctx context.Context) error {
		body, err := c.PlainTextCall(ctx, requestsBody)
		if err != nil {
			return err
		}

		var rpcResponse []map[string]json.RawMessage
		if err := json.Unmarshal(body, &rpcResponse); err != nil {
			var rpcErrorResp map[string]json.RawMessage
			if json.Unmarshal(body, &rpcErrorResp) == nil {
				if errorMsg, ok := rpcErrorResp["error"]; ok {
					return fmt.Errorf("%w: %s", ErrRPCError, errorMsg)
				}
			}

			c.logger.Debug().Err(err).Str("response", string(body)).Msg("failed to unmarshal response")
			return fmt.Errorf("%w: %w", ErrFailedToUnmarshalResponse, err)
		}
		c.logger.Trace().RawJSON("response", body).Send()

		results = make([]json.RawMessage, len(rpcResponse))
		for i, resp := range rpcResponse {
			if errorMsg, ok := resp["error"]; ok {
				return fmt.Errorf("%w: %s (%d)", ErrRPCError, errorMsg, i)
			}
			results[i] = resp["result"]
		}
		return nil
	}

	if c.retrier != nil {
		err = c.retrier.Do(ctx, call)
	} else {
		err = call(ctx)
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

func SendRequest(
	ctx context.Context, client http.Client, endpoint string, requestBody []byte, headers map[string]string,
) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
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

func (c *Client) PlainTextCall(ctx context.Context, requestBody []byte) (json.RawMessage, error) {
	c.logger.Trace().RawJSON("request", requestBody).Send()

	return SendRequest(ctx, c.client, c.endpoint, requestBody, c.headers)
}

func (c *Client) RawCall(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return c.call(ctx, method, params...)
}

func (c *Client) GetCode(ctx context.Context, addr types.Address, blockId any) (types.Code, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Code{}, err
	}
	codeHex, err := simpleCall[string](ctx, c, Eth_getCode, addr, blockNrOrHash)
	if err != nil {
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
		request = c.newRequest(m, *blockNrOrHash.BlockHash, fullTx)
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

func (c *Client) getSendTransactionViaSmartContractRequest(ctx context.Context, smartAccountAddress types.Address,
	bytecode types.Code, fee types.FeePack, value types.Value, tokens []types.TokenBalance,
	contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool, id int,
) (*Request, error) {
	calldataExt, err := client.CreateInternalTransactionPayload(bytecode, value, tokens, contractAddress, isDeploy)
	if err != nil {
		return nil, err
	}

	extTxn, err := client.CreateExternalTransaction(ctx, c, calldataExt, smartAccountAddress, fee, isDeploy, id)
	if err != nil {
		return nil, err
	}

	if pk != nil {
		err = extTxn.Sign(pk)
		if err != nil {
			return nil, err
		}
	}

	if len(extTxn.Data) > types.TransactionMaxDataSize {
		return nil, ErrTxnDataTooLong
	}
	data, err := extTxn.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	request := c.newRequest(Eth_sendRawTransaction, hexutil.Bytes(data))
	return request, nil
}

func (c *Client) GetBlock(
	ctx context.Context,
	shardId types.ShardId,
	blockId any,
	fullTx bool,
) (*jsonrpc.RPCBlock, error) {
	request, err := c.getBlockRequest(shardId, blockId, fullTx, false)
	if err != nil {
		return nil, err
	}

	res, err := c.performRequest(ctx, request)
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

func (c *Client) GetDebugBlock(
	ctx context.Context,
	shardId types.ShardId,
	blockId any,
	fullTx bool,
) (*jsonrpc.DebugRPCBlock, error) {
	request, err := c.getBlockRequest(shardId, blockId, fullTx, true)
	if err != nil {
		return nil, err
	}

	res, err := c.performRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	return toRawBlock(res)
}

func toRawBlock(raw json.RawMessage) (*jsonrpc.DebugRPCBlock, error) {
	var blockInfo *jsonrpc.DebugRPCBlock
	if err := json.Unmarshal(raw, &blockInfo); err != nil {
		return nil, err
	}
	return blockInfo, nil
}

func (c *Client) SendTransaction(ctx context.Context, txn *types.ExternalTransaction) (common.Hash, error) {
	if len(txn.Data) > types.TransactionMaxDataSize {
		return common.EmptyHash, ErrTxnDataTooLong
	}
	data, err := txn.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendRawTransaction(ctx, data)
}

func (c *Client) SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error) {
	return simpleCall[common.Hash](ctx, c, Eth_sendRawTransaction, hexutil.Bytes(data))
}

func (c *Client) GetInTransactionByHash(ctx context.Context, hash common.Hash) (*jsonrpc.RPCInTransaction, error) {
	return simpleCall[*jsonrpc.RPCInTransaction](ctx, c, Eth_getInTransactionByHash, hash)
}

func (c *Client) GetInTransactionReceipt(ctx context.Context, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return simpleCall[*jsonrpc.RPCReceipt](ctx, c, Eth_getInTransactionReceipt, hash)
}

func (c *Client) GetTransactionCount(ctx context.Context, address types.Address, blockId any) (types.Seqno, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	return simpleCallUint64[types.Seqno](
		ctx, c, Eth_getTransactionCount, address, transport.BlockNumberOrHash(blockNrOrHash))
}

func toString(raw json.RawMessage) string {
	res := strings.TrimSpace(string(raw))
	if len(res) >= 2 && res[0] == '"' && res[len(res)-1] == '"' {
		res = res[1 : len(res)-1]
	}
	return res
}

func toUint64(raw json.RawMessage) (uint64, error) {
	return strconv.ParseUint(toString(raw), 0, 64)
}

func (c *Client) GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockId any) (uint64, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.getBlockTransactionCountByHash(ctx, *blockNrOrHash.BlockHash)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.getBlockTransactionCountByNumber(ctx, shardId, *blockNrOrHash.BlockNumber)
	}
	if assert.Enable {
		panic("Unreachable")
	}
	return 0, nil
}

func (c *Client) getBlockTransactionCountByNumber(
	ctx context.Context,
	shardId types.ShardId,
	number transport.BlockNumber,
) (uint64, error) {
	return simpleCallUint64[uint64](ctx, c, Eth_getBlockTransactionCountByNumber, shardId, number)
}

func (c *Client) getBlockTransactionCountByHash(ctx context.Context, hash common.Hash) (uint64, error) {
	return simpleCallUint64[uint64](ctx, c, Eth_getBlockTransactionCountByHash, hash)
}

func (c *Client) GetBalance(ctx context.Context, address types.Address, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}

	bigVal, err := simpleCall[hexutil.Big](ctx, c, Eth_getBalance, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return types.Value{}, err
	}
	return types.NewValueFromBigMust(bigVal.ToInt()), nil
}

func (c *Client) GetTokens(ctx context.Context, address types.Address, blockId any) (types.TokensMap, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return simpleCall[types.TokensMap](ctx, c, Eth_getTokens, address, transport.BlockNumberOrHash(blockNrOrHash))
}

func (c *Client) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	return simpleCall[types.Value](ctx, c, Eth_gasPrice, shardId)
}

func (c *Client) ChainId(ctx context.Context) (types.ChainId, error) {
	res, err := c.call(ctx, Eth_chainId)
	if err != nil {
		return types.ChainId(0), err
	}

	val, err := toUint64(res)
	if err != nil {
		return types.ChainId(0), err
	}
	return types.ChainId(val), err
}

func (c *Client) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return simpleCall[[]types.ShardId](ctx, c, Eth_getShardIdList)
}

func (c *Client) GetNumShards(ctx context.Context) (uint64, error) {
	return simpleCallUint64[uint64](ctx, c, Eth_getNumShards)
}

func (c *Client) ClientVersion(ctx context.Context) (string, error) {
	res, err := c.call(ctx, Web3_clientVersion)
	if err != nil {
		return "", err
	}
	return toString(res), nil
}

func (c *Client) DeployContract(
	ctx context.Context,
	shardId types.ShardId,
	smartAccountAddress types.Address,
	payload types.DeployPayload,
	value types.Value,
	fee types.FeePack,
	pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := client.SendTransactionViaSmartAccount(ctx, c, smartAccountAddress, payload.Bytes(), fee,
		value, []types.TokenBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *Client) DeployExternal(
	ctx context.Context,
	shardId types.ShardId,
	deployPayload types.DeployPayload,
	fee types.FeePack,
) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := client.SendExternalTransaction(ctx, c, deployPayload.Bytes(), address, nil, fee, true, false)
	return msgHash, address, err
}

func (c *Client) SendTransactionViaSmartAccount(
	ctx context.Context,
	smartAccountAddress types.Address,
	bytecode types.Code,
	fee types.FeePack,
	value types.Value,
	tokens []types.TokenBalance,
	contractAddress types.Address,
	pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return client.SendTransactionViaSmartAccount(
		ctx, c, smartAccountAddress, bytecode, fee, value, tokens, contractAddress, pk, false)
}

func (c *Client) SendExternalTransaction(
	ctx context.Context, bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, fee types.FeePack,
) (common.Hash, error) {
	return client.SendExternalTransaction(ctx, c, bytecode, contractAddress, pk, fee, false, false)
}

func (c *Client) Call(
	ctx context.Context,
	args *jsonrpc.CallArgs,
	blockId any,
	stateOverride *jsonrpc.StateOverrides,
) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return simpleCall[*jsonrpc.CallRes](ctx, c, Eth_call, args, blockNrOrHash, stateOverride)
}

func (c *Client) EstimateFee(
	ctx context.Context,
	args *jsonrpc.CallArgs,
	blockId any,
) (*jsonrpc.EstimateFeeRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return simpleCall[*jsonrpc.EstimateFeeRes](ctx, c, Eth_estimateFee, args, blockNrOrHash)
}

func (c *Client) SetTokenName(
	ctx context.Context,
	contractAddr types.Address,
	name string,
	pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilTokenBase, "setTokenName", name)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalTransaction(ctx, data, contractAddr, pk, types.NewFeePackFromGas(100_000))
}

func (c *Client) ChangeTokenAmount(
	ctx context.Context,
	contractAddr types.Address,
	amount types.Value,
	pk *ecdsa.PrivateKey,
	mint bool,
) (common.Hash, error) {
	method := "mintToken"
	if !mint {
		method = "burnToken"
	}
	data, err := contracts.NewCallData(contracts.NameNilTokenBase, method, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalTransaction(ctx, data, contractAddr, pk, types.NewFeePackFromGas(100_000))
}

func callDbAPI[T any](ctx context.Context, c *Client, method string, params ...any) (T, error) {
	var res T
	raw, err := c.call(ctx, method, params...)
	if err != nil {
		if strings.Contains(err.Error(), jsonrpc.ErrApiKeyNotFound.Error()) {
			return res, db.ErrKeyNotFound
		}
		return res, err
	}

	return res, json.Unmarshal(raw, &res)
}

func (c *Client) DbInitTimestamp(ctx context.Context, ts uint64) error {
	_, err := c.call(ctx, Db_initDbTimestamp, ts)
	return err
}

func (c *Client) DbGet(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error) {
	return callDbAPI[[]byte](ctx, c, Db_get, tableName, key)
}

func (c *Client) DbGetFromShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) ([]byte, error) {
	return callDbAPI[[]byte](ctx, c, Db_getFromShard, shardId, tableName, key)
}

func (c *Client) DbExists(ctx context.Context, tableName db.TableName, key []byte) (bool, error) {
	return callDbAPI[bool](ctx, c, Db_exists, tableName, key)
}

func (c *Client) DbExistsInShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) (bool, error) {
	return callDbAPI[bool](ctx, c, Db_existsInShard, shardId, tableName, key)
}

func (c *Client) CreateBatchRequest() client.BatchRequest {
	return &BatchRequestImpl{
		requests: make([]*Request, 0),
		client:   c,
	}
}

func (c *Client) BatchCall(ctx context.Context, req client.BatchRequest) ([]any, error) {
	r, ok := req.(*BatchRequestImpl)
	check.PanicIfNot(ok)

	responses, err := c.performRequests(ctx, r.requests)
	if err != nil {
		return nil, err
	}
	if len(responses) != len(r.requests) {
		return nil, fmt.Errorf("unexpected number of responses: expected %d, got %d", len(r.requests), len(responses))
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
			if block != nil {
				result[i] = block
			}
		case Debug_getBlockByHash, Debug_getBlockByNumber:
			block, err := toRawBlock(resp)
			if err != nil {
				return nil, err
			}
			if block != nil {
				result[i] = block
			}
		default:
			result[i] = resp
		}
	}

	return result, nil
}

func (c *Client) fetchBlocksBatch(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	isDebug bool,
) ([]any, error) {
	batch := c.CreateBatchRequest()

	for i := from; i < to; i++ {
		var err error
		if isDebug {
			_, err = batch.GetDebugBlock(shardId, transport.BlockNumber(i), fullTx)
		} else {
			_, err = batch.GetBlock(shardId, transport.BlockNumber(i), fullTx)
		}
		if err != nil {
			return nil, err
		}
	}

	return c.BatchCall(ctx, batch)
}

func (c *Client) getBlocksRange(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	batchSize int,
	isDebug bool,
) ([]any, error) {
	if from >= to {
		return nil, nil
	}

	result := make([]any, 0)
	for curBlockId := from; curBlockId < to; curBlockId += types.BlockNumber(batchSize) {
		batchEndId := curBlockId + types.BlockNumber(batchSize)
		if batchEndId > to {
			batchEndId = to
		}

		blocks, err := c.fetchBlocksBatch(ctx, shardId, curBlockId, batchEndId, fullTx, isDebug)
		if err != nil {
			return nil, err
		}
		for _, block := range blocks {
			if block != nil {
				result = append(result, block)
			}
		}
	}
	return result, nil
}

func (c *Client) GetDebugBlocksRange(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	batchSize int,
) ([]*jsonrpc.DebugRPCBlock, error) {
	rawBlocks, err := c.getBlocksRange(ctx, shardId, from, to, fullTx, batchSize, true)
	if err != nil {
		return nil, err
	}

	result := make([]*jsonrpc.DebugRPCBlock, len(rawBlocks))
	for i, raw := range rawBlocks {
		var ok bool
		result[i], ok = raw.(*jsonrpc.DebugRPCBlock)
		check.PanicIfNot(ok)
	}
	return result, nil
}

func (c *Client) GetBlocksRange(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	batchSize int,
) ([]*jsonrpc.RPCBlock, error) {
	rawBlocks, err := c.getBlocksRange(ctx, shardId, from, to, fullTx, batchSize, false)
	if err != nil {
		return nil, err
	}

	result := make([]*jsonrpc.RPCBlock, len(rawBlocks))
	for i, raw := range rawBlocks {
		var ok bool
		result[i], ok = raw.(*jsonrpc.RPCBlock)
		check.PanicIfNot(ok)
	}
	return result, nil
}

func (c *Client) GetDebugContract(
	ctx context.Context,
	contractAddr types.Address,
	blockId any,
) (*jsonrpc.DebugRPCContract, error) {
	blockRef, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	return simpleCall[*jsonrpc.DebugRPCContract](ctx, c, Debug_getContract, contractAddr, blockRef)
}

func (c *Client) DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error) {
	_, err := c.call(ctx, Dev_doPanicOnShard, shardId)
	return 0, err
}

func simpleCall[ReturnType any](ctx context.Context, c *Client, method string, params ...any) (ReturnType, error) {
	res, err := c.call(ctx, method, params...)
	var result ReturnType
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(res, &result)
	return result, err
}

func simpleCallUint64[ReturnType ~uint64](
	ctx context.Context,
	c *Client,
	method string,
	params ...any,
) (ReturnType, error) {
	res, err := c.call(ctx, method, params...)
	if err != nil {
		return 0, err
	}
	result, err := toUint64(res)
	if err != nil {
		return 0, err
	}
	return ReturnType(result), err
}

func (c *Client) GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (jsonrpc.TxPoolStatus, error) {
	return simpleCall[jsonrpc.TxPoolStatus](ctx, c, Txpool_getTxpoolStatus, shardId)
}

func (c *Client) GetTxpoolContent(ctx context.Context, shardId types.ShardId) (jsonrpc.TxPoolContent, error) {
	return simpleCall[jsonrpc.TxPoolContent](ctx, c, Txpool_getTxpoolContent, shardId)
}
