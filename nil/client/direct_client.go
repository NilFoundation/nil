package client

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// DirectClient is a client that interacts with the end api directly, without using the rpc server.
type DirectClient struct {
	ethApi   jsonrpc.EthAPI
	debugApi jsonrpc.DebugAPI
	dbApi    jsonrpc.DbAPI
	web3Api  jsonrpc.Web3API
	devApi   jsonrpc.DevAPI
}

var _ Client = (*DirectClient)(nil)

func NewEthClient(
	ctx context.Context,
	db db.ReadOnlyDB,
	localApi *rawapi.NodeApiOverShardApis,
	logger logging.Logger,
) (*DirectClient, error) {
	ethApi := jsonrpc.NewEthAPI(ctx, localApi, db, true, false)
	debugApi := jsonrpc.NewDebugAPI(localApi, logger)
	dbApi := jsonrpc.NewDbAPI(db, logger)
	web3Api := jsonrpc.NewWeb3API(localApi)
	devApi := jsonrpc.NewDevAPI(localApi)

	return &DirectClient{
		ethApi:   ethApi,
		debugApi: debugApi,
		dbApi:    dbApi,
		web3Api:  web3Api,
		devApi:   devApi,
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

func (c *DirectClient) GetBlock(
	ctx context.Context,
	shardId types.ShardId,
	blockId any,
	fullTx bool,
) (*jsonrpc.RPCBlock, error) {
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

func (c *DirectClient) GetDebugBlock(
	ctx context.Context,
	shardId types.ShardId,
	blockId any,
	fullTx bool,
) (*jsonrpc.DebugRPCBlock, error) {
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

func (c *DirectClient) GetDebugBlocksRange(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	batchSize int,
) ([]*jsonrpc.DebugRPCBlock, error) {
	panic("Not supported")
}

func (c *DirectClient) GetBlocksRange(
	ctx context.Context,
	shardId types.ShardId,
	from types.BlockNumber,
	to types.BlockNumber,
	fullTx bool,
	batchSize int,
) ([]*jsonrpc.RPCBlock, error) {
	panic("Not supported")
}

func (c *DirectClient) SendTransaction(ctx context.Context, txn *types.ExternalTransaction) (common.Hash, error) {
	data, err := txn.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendRawTransaction(ctx, data)
}

func (c *DirectClient) SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error) {
	return c.ethApi.SendRawTransaction(ctx, data)
}

func (c *DirectClient) GetInTransactionByHash(
	ctx context.Context,
	hash common.Hash,
) (*jsonrpc.RPCInTransaction, error) {
	return c.ethApi.GetInTransactionByHash(ctx, hash)
}

func (c *DirectClient) GetInTransactionReceipt(ctx context.Context, hash common.Hash) (*jsonrpc.RPCReceipt, error) {
	return c.ethApi.GetInTransactionReceipt(ctx, hash)
}

func (c *DirectClient) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockId any,
) (types.Seqno, error) {
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

func (c *DirectClient) GetBlockTransactionCount(
	ctx context.Context,
	shardId types.ShardId,
	blockId any,
) (uint64, error) {
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

func (c *DirectClient) GetTokens(ctx context.Context, address types.Address, blockId any) (types.TokensMap, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}

	return c.ethApi.GetTokens(ctx, address, transport.BlockNumberOrHash(blockNrOrHash))
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

func (c *DirectClient) GetNumShards(ctx context.Context) (uint64, error) {
	return c.ethApi.GetNumShards(ctx)
}

func (c *DirectClient) DeployContract(
	ctx context.Context, shardId types.ShardId, smartAccountAddress types.Address, payload types.DeployPayload,
	value types.Value, fee types.FeePack, pk *ecdsa.PrivateKey,
) (common.Hash, types.Address, error) {
	contractAddr := types.CreateAddress(shardId, payload)
	txnHash, err := SendTransactionViaSmartAccount(ctx, c, smartAccountAddress, payload.Bytes(), fee, value,
		[]types.TokenBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txnHash, contractAddr, nil
}

func (c *DirectClient) DeployExternal(ctx context.Context, shardId types.ShardId, deployPayload types.DeployPayload,
	fee types.FeePack,
) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	txnHash, err := SendExternalTransaction(ctx, c, deployPayload.Bytes(), address, nil, fee, true, false)
	return txnHash, address, err
}

func (c *DirectClient) SendTransactionViaSmartAccount(
	ctx context.Context, smartAccountAddress types.Address, bytecode types.Code, fee types.FeePack, value types.Value,
	tokens []types.TokenBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return SendTransactionViaSmartAccount(
		ctx, c, smartAccountAddress, bytecode, fee, value, tokens, contractAddress, pk, false)
}

func (c *DirectClient) SendExternalTransaction(
	ctx context.Context, bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, fee types.FeePack,
) (common.Hash, error) {
	return SendExternalTransaction(ctx, c, bytecode, contractAddress, pk, fee, false, false)
}

func (c *DirectClient) Call(
	ctx context.Context,
	args *jsonrpc.CallArgs,
	blockId any,
	stateOverride *jsonrpc.StateOverrides,
) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return c.ethApi.Call(ctx, *args, transport.BlockNumberOrHash(blockNrOrHash), stateOverride)
}

func (c *DirectClient) EstimateFee(
	ctx context.Context,
	args *jsonrpc.CallArgs,
	blockId any,
) (*jsonrpc.EstimateFeeRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return c.ethApi.EstimateFee(ctx, *args, transport.BlockNumberOrHash(blockNrOrHash))
}

func (c *DirectClient) RawCall(_ context.Context, method string, params ...any) (json.RawMessage, error) {
	panic("Not supported")
}

func (c *DirectClient) SetTokenName(
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

func (c *DirectClient) ChangeTokenAmount(
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

func (c *DirectClient) DbInitTimestamp(ctx context.Context, ts uint64) error {
	return c.dbApi.InitDbTimestamp(ctx, ts)
}

func (c *DirectClient) DbGet(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error) {
	return c.dbApi.Get(ctx, tableName, key)
}

func (c *DirectClient) DbGetFromShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) ([]byte, error) {
	return c.dbApi.GetFromShard(ctx, shardId, tableName, key)
}

func (c *DirectClient) DbExists(ctx context.Context, tableName db.TableName, key []byte) (bool, error) {
	return c.dbApi.Exists(ctx, tableName, key)
}

func (c *DirectClient) DbExistsInShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) (bool, error) {
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

func (c *DirectClient) GetDebugContract(
	ctx context.Context,
	contractAddr types.Address,
	blockId any,
) (*jsonrpc.DebugRPCContract, error) {
	panic("Not supported")
}

func (c *DirectClient) ClientVersion(ctx context.Context) (string, error) {
	return c.web3Api.ClientVersion(ctx)
}

func (c *DirectClient) DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error) {
	return c.devApi.DoPanicOnShard(ctx, shardId)
}
