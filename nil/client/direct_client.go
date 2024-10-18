//go:build test

package client

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"sync"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

// DirectClient is a client that interacts with the end api directly, without using the rpc server.
type DirectClient struct {
	ethApi   jsonrpc.EthAPI
	debugApi jsonrpc.DebugAPI
	dbApi    jsonrpc.DbAPI
	ctx      context.Context
}

var _ Client = (*DirectClient)(nil)

func NewEthClient(ctx context.Context, wg *sync.WaitGroup, db db.ReadOnlyDB, nShards types.ShardId, msgPools map[types.ShardId]msgpool.Pool, logger zerolog.Logger) (*DirectClient, error) {
	var err error
	localShardApis := make(map[types.ShardId]rawapi.ShardApi)
	for shardId := range nShards {
		localShardApis[shardId], err = rawapi.NewLocalRawApiAccessor(shardId, rawapi.NewLocalShardApi(shardId, db, msgPools[shardId]))
		if err != nil {
			return nil, err
		}
	}
	localApi := rawapi.NewNodeApiOverShardApis(localShardApis)

	ethApi := jsonrpc.NewEthAPI(ctx, localApi, db, true)
	debugApi := jsonrpc.NewDebugAPI(localApi, db, logger)
	dbApi := jsonrpc.NewDbAPI(db, logger)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		<-ctx.Done()
		ethApi.Shutdown()
		wg.Done()
	}(wg)

	c := &DirectClient{
		ethApi:   ethApi,
		debugApi: debugApi,
		dbApi:    dbApi,
		ctx:      ctx,
	}

	return c, nil
}

func (c *DirectClient) GetCode(addr types.Address, blockId any) (types.Code, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Code{}, err
	}

	raw, err := c.ethApi.GetCode(c.ctx, addr, transport.BlockNumberOrHash(blockNrOrHash))

	return types.Code(raw), err
}

func (c *DirectClient) GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.ethApi.GetBlockByHash(c.ctx, shardId, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.ethApi.GetBlockByNumber(c.ctx, shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	if assert.Enable {
		panic("Unreachable")
	}

	return nil, nil
}

func (c *DirectClient) GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.DebugRPCBlock, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.debugApi.GetBlockByHash(c.ctx, shardId, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.debugApi.GetBlockByNumber(c.ctx, shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	if assert.Enable {
		panic("Unreachable")
	}

	return nil, nil
}

func (c *DirectClient) GetDebugBlocksRange(shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*types.BlockWithExtractedData, error) {
	panic("Not supported")
}

func (c *DirectClient) GetBlocksRange(shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.RPCBlock, error) {
	panic("Not supported")
}

func (c *DirectClient) SendMessage(msg *types.ExternalMessage) (common.Hash, error) {
	data, err := msg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendRawTransaction(data)
}

func (c *DirectClient) SendRawTransaction(data []byte) (common.Hash, error) {
	return c.ethApi.SendRawTransaction(c.ctx, data)
}

func (c *DirectClient) GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error) {
	return c.ethApi.GetInMessageByHash(c.ctx, shardId, hash)
}

func (c *DirectClient) GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return c.ethApi.GetInMessageReceipt(c.ctx, shardId, hash)
}

func (c *DirectClient) GetTransactionCount(address types.Address, blockId any) (types.Seqno, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	res, err := c.ethApi.GetTransactionCount(c.ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return 0, err
	}

	return types.Seqno(res), nil
}

func (c *DirectClient) GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	var res hexutil.Uint

	switch {
	case blockNrOrHash.BlockHash != nil:
		res, err = c.ethApi.GetBlockTransactionCountByHash(c.ctx, shardId, *blockNrOrHash.BlockHash)
	case blockNrOrHash.BlockNumber != nil:
		res, err = c.ethApi.GetBlockTransactionCountByNumber(c.ctx, shardId, *blockNrOrHash.BlockNumber)
	default:
		if assert.Enable {
			panic("Unreachable")
		}
	}

	return uint64(res), err
}

func (c *DirectClient) GetBalance(address types.Address, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}

	res, err := c.ethApi.GetBalance(c.ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return types.Value{}, err
	}

	return types.NewValueFromBigMust(res.ToInt()), nil
}

func (c *DirectClient) GetCurrencies(address types.Address, blockId any) (types.CurrenciesMap, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	currencies, err := c.ethApi.GetCurrencies(c.ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return nil, err
	}

	return types.ToCurrenciesMap(currencies), err
}

func (c *DirectClient) GasPrice(shardId types.ShardId) (types.Value, error) {
	return c.ethApi.GasPrice(c.ctx, shardId)
}

func (c *DirectClient) ChainId() (types.ChainId, error) {
	res, err := c.ethApi.ChainId(c.ctx)
	if err != nil {
		return types.ChainId(0), err
	}

	return types.ChainId(res), err
}

func (c *DirectClient) GetShardIdList() ([]types.ShardId, error) {
	return c.ethApi.GetShardIdList(c.ctx)
}

func (c *DirectClient) DeployContract(
	shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := SendMessageViaWallet(c, walletAddress, payload.Bytes(), types.GasToValue(100_000), value,
		[]types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *DirectClient) DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := SendExternalMessage(c, deployPayload.Bytes(), address, nil, feeCredit, true, false)
	return msgHash, address, err
}

func (c *DirectClient) SendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return SendMessageViaWallet(c, walletAddress, bytecode, feeCredit, value, currencies, contractAddress, pk, false)
}

func (c *DirectClient) SendExternalMessage(
	bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, feeCredit types.Value,
) (common.Hash, error) {
	return SendExternalMessage(c, bytecode, contractAddress, pk, feeCredit, false, false)
}

func (c *DirectClient) TopUpViaFaucet(faucetAddress, contractAddressTo types.Address, amount types.Value) (common.Hash, error) {
	return c.ethApi.TopUpViaFaucet(c.ctx, faucetAddress, contractAddressTo, amount)
}

func (c *DirectClient) Call(args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return c.ethApi.Call(c.ctx, *args, transport.BlockNumberOrHash(blockNrOrHash), stateOverride)
}

func (c *DirectClient) EstimateFee(args *jsonrpc.CallArgs, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}
	return c.ethApi.EstimateFee(c.ctx, *args, transport.BlockNumberOrHash(blockNrOrHash))
}

func (c *DirectClient) RawCall(method string, params ...any) (json.RawMessage, error) {
	panic("Not supported")
}

func (c *DirectClient) SetCurrencyName(contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "setCurrencyName", name)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk, types.GasToValue(100_000))
}

func (c *DirectClient) ChangeCurrencyAmount(contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey, mint bool) (common.Hash, error) {
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

func (c *DirectClient) DbInitTimestamp(ts uint64) error {
	return c.dbApi.InitDbTimestamp(c.ctx, ts)
}

func (c *DirectClient) DbGet(tableName db.TableName, key []byte) ([]byte, error) {
	return c.dbApi.Get(c.ctx, tableName, key)
}

func (c *DirectClient) DbGetFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	return c.dbApi.GetFromShard(c.ctx, shardId, tableName, key)
}

func (c *DirectClient) DbExists(tableName db.TableName, key []byte) (bool, error) {
	return c.dbApi.Exists(c.ctx, tableName, key)
}

func (c *DirectClient) DbExistsInShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	return c.dbApi.ExistsInShard(c.ctx, shardId, tableName, key)
}

func (c *DirectClient) CreateBatchRequest() BatchRequest {
	panic("Not supported")
}

func (c *DirectClient) BatchCall(BatchRequest) ([]any, error) {
	panic("Not supported")
}

func (c *DirectClient) PlainTextCall(requestBody []byte) (json.RawMessage, error) {
	panic("Not supported")
}
