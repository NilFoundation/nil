package client

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"

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
}

var _ Client = (*DirectClient)(nil)

func NewEthClient(ctx context.Context, db db.ReadOnlyDB, nShards types.ShardId, msgPools map[types.ShardId]msgpool.Pool, logger zerolog.Logger) (*DirectClient, error) {
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
	debugApi := jsonrpc.NewDebugAPI(localApi, logger)
	dbApi := jsonrpc.NewDbAPI(db, logger)

	return &DirectClient{
		ethApi:   ethApi,
		debugApi: debugApi,
		dbApi:    dbApi,
	}, nil
}

func (c *DirectClient) GetCode(ctx context.Context, addr types.Address, blockId any) (types.Code, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Code{}, err
	}

	raw, err := c.ethApi.GetCode(ctx, addr, transport.BlockNumberOrHash(blockNrOrHash))

	return types.Code(raw), err
}

func (c *DirectClient) GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.ethApi.GetBlockByHash(ctx, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.ethApi.GetBlockByNumber(ctx, shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	if assert.Enable {
		panic("Unreachable")
	}

	return nil, nil
}

func (c *DirectClient) GetDebugBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.DebugRPCBlock, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockHash != nil {
		return c.debugApi.GetBlockByHash(ctx, *blockNrOrHash.BlockHash, fullTx)
	}
	if blockNrOrHash.BlockNumber != nil {
		return c.debugApi.GetBlockByNumber(ctx, shardId, *blockNrOrHash.BlockNumber, fullTx)
	}
	if assert.Enable {
		panic("Unreachable")
	}

	return nil, nil
}

func (c *DirectClient) GetDebugBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.DebugRPCBlock, error) {
	panic("Not supported")
}

func (c *DirectClient) GetBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.RPCBlock, error) {
	panic("Not supported")
}

func (c *DirectClient) SendMessage(ctx context.Context, msg *types.ExternalMessage) (common.Hash, error) {
	data, err := msg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendRawTransaction(ctx, data)
}

func (c *DirectClient) SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error) {
	return c.ethApi.SendRawTransaction(ctx, data)
}

func (c *DirectClient) GetInMessageByHash(ctx context.Context, hash common.Hash) (*jsonrpc.RPCInMessage, error) {
	return c.ethApi.GetInMessageByHash(ctx, hash)
}

func (c *DirectClient) GetInMessageReceipt(ctx context.Context, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return c.ethApi.GetInMessageReceipt(ctx, hash)
}

func (c *DirectClient) GetTransactionCount(ctx context.Context, address types.Address, blockId any) (types.Seqno, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	res, err := c.ethApi.GetTransactionCount(ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return 0, err
	}

	return types.Seqno(res), nil
}

func (c *DirectClient) GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockId any) (uint64, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return 0, err
	}

	var res hexutil.Uint

	switch {
	case blockNrOrHash.BlockHash != nil:
		res, err = c.ethApi.GetBlockTransactionCountByHash(ctx, *blockNrOrHash.BlockHash)
	case blockNrOrHash.BlockNumber != nil:
		res, err = c.ethApi.GetBlockTransactionCountByNumber(ctx, shardId, *blockNrOrHash.BlockNumber)
	default:
		if assert.Enable {
			panic("Unreachable")
		}
	}

	return uint64(res), err
}

func (c *DirectClient) GetBalance(ctx context.Context, address types.Address, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}

	res, err := c.ethApi.GetBalance(ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
	if err != nil {
		return types.Value{}, err
	}

	return types.NewValueFromBigMust(res.ToInt()), nil
}

func (c *DirectClient) GetCurrencies(ctx context.Context, address types.Address, blockId any) (types.CurrenciesMap, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	return c.ethApi.GetCurrencies(ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
}

func (c *DirectClient) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	return c.ethApi.GasPrice(ctx, shardId)
}

func (c *DirectClient) ChainId(ctx context.Context) (types.ChainId, error) {
	res, err := c.ethApi.ChainId(ctx)
	if err != nil {
		return types.ChainId(0), err
	}

	return types.ChainId(res), err
}

func (c *DirectClient) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return c.ethApi.GetShardIdList(ctx)
}

func (c *DirectClient) DeployContract(
	ctx context.Context, shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := SendMessageViaWallet(ctx, c, walletAddress, payload.Bytes(), types.GasToValue(100_000), value,
		[]types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *DirectClient) DeployExternal(ctx context.Context, shardId types.ShardId, deployPayload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := SendExternalMessage(ctx, c, deployPayload.Bytes(), address, nil, feeCredit, true, false)
	return msgHash, address, err
}

func (c *DirectClient) SendMessageViaWallet(
	ctx context.Context, walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return SendMessageViaWallet(ctx, c, walletAddress, bytecode, feeCredit, value, currencies, contractAddress, pk, false)
}

func (c *DirectClient) SendExternalMessage(
	ctx context.Context, bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, feeCredit types.Value,
) (common.Hash, error) {
	return SendExternalMessage(ctx, c, bytecode, contractAddress, pk, feeCredit, false, false)
}

func (c *DirectClient) Call(ctx context.Context, args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return c.ethApi.Call(ctx, *args, transport.BlockNumberOrHash(blockNrOrHash), stateOverride)
}

func (c *DirectClient) EstimateFee(ctx context.Context, args *jsonrpc.CallArgs, blockId any) (types.Value, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return types.Value{}, err
	}
	return c.ethApi.EstimateFee(ctx, *args, transport.BlockNumberOrHash(blockNrOrHash))
}

func (c *DirectClient) RawCall(_ context.Context, method string, params ...any) (json.RawMessage, error) {
	panic("Not supported")
}

func (c *DirectClient) SetCurrencyName(ctx context.Context, contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "setCurrencyName", name)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(ctx, data, contractAddr, pk, types.GasToValue(100_000))
}

func (c *DirectClient) ChangeCurrencyAmount(ctx context.Context, contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey, mint bool) (common.Hash, error) {
	method := "mintCurrency"
	if !mint {
		method = "burnCurrency"
	}
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, method, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(ctx, data, contractAddr, pk, types.GasToValue(100_000))
}

func (c *DirectClient) DbInitTimestamp(ctx context.Context, ts uint64) error {
	return c.dbApi.InitDbTimestamp(ctx, ts)
}

func (c *DirectClient) DbGet(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error) {
	return c.dbApi.Get(ctx, tableName, key)
}

func (c *DirectClient) DbGetFromShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	return c.dbApi.GetFromShard(ctx, shardId, tableName, key)
}

func (c *DirectClient) DbExists(ctx context.Context, tableName db.TableName, key []byte) (bool, error) {
	return c.dbApi.Exists(ctx, tableName, key)
}

func (c *DirectClient) DbExistsInShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	return c.dbApi.ExistsInShard(ctx, shardId, tableName, key)
}

func (c *DirectClient) CreateBatchRequest() BatchRequest {
	panic("Not supported")
}

func (c *DirectClient) BatchCall(ctx context.Context, _ BatchRequest) ([]any, error) {
	panic("Not supported")
}

func (c *DirectClient) PlainTextCall(_ context.Context, requestBody []byte) (json.RawMessage, error) {
	panic("Not supported")
}

func (c *DirectClient) GetDebugContract(ctx context.Context, contractAddr types.Address, blockId any) (*jsonrpc.DebugRPCContract, error) {
	panic("Not supported")
}
