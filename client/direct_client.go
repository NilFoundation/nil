//go:build test

package client

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/assert"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
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

func NewEthClient(ctx context.Context, db db.ReadOnlyDB, msgPools []msgpool.Pool, logger zerolog.Logger) (*DirectClient, error) {
	ethApi, err := jsonrpc.NewEthAPI(ctx, nil, db, msgPools, logger)
	if err != nil {
		return nil, err
	}
	debugApi := jsonrpc.NewDebugAPI(nil, db, logger)
	dbApi := jsonrpc.NewDbAPI(db, logger)

	c := &DirectClient{
		ethApi:   ethApi,
		debugApi: debugApi,
		dbApi:    dbApi,
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

func (c *DirectClient) GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.HexedDebugRPCBlock, error) {
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

	return *(*types.Seqno)(res), nil
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
	res, err := c.ethApi.GasPrice(c.ctx, shardId)
	if err != nil {
		return types.Value{}, err
	}
	return types.NewValueFromBigMust(res.ToInt()), nil
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
	txHash, err := c.sendMessageViaWallet(walletAddress, payload.Bytes(), types.GasToValue(100_000), value, []types.CurrencyBalance{}, contractAddr, pk, true)
	if err != nil {
		return common.EmptyHash, types.EmptyAddress, err
	}
	return txHash, contractAddr, nil
}

func (c *DirectClient) DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload) (common.Hash, types.Address, error) {
	address := types.CreateAddress(shardId, deployPayload)
	msgHash, err := c.sendExternalMessage(deployPayload.Bytes(), address, nil, true)
	return msgHash, address, err
}

func (c *DirectClient) SendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, feeCredit types.Value, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return c.sendMessageViaWallet(walletAddress, bytecode, feeCredit, value, currencies, contractAddress, pk, false)
}

// RunContract runs bytecode on the specified contract address
func (c *DirectClient) sendMessageViaWallet(
	walletAddress types.Address, bytecode types.Code, feeCredit types.Value, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	intMsg := &types.InternalMessagePayload{
		Data:        bytecode,
		To:          contractAddress,
		Value:       value,
		FeeCredit:   feeCredit,
		ForwardKind: types.ForwardKindNone,
		Currency:    currencies,
		Kind:        kind,
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

func (c *DirectClient) SendExternalMessage(
	bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	return c.sendExternalMessage(bytecode, contractAddress, pk, false)
}

func (c *DirectClient) sendExternalMessage(
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

func (c *DirectClient) TopUpViaFaucet(contractAddress types.Address, amount types.Value) (common.Hash, error) {
	callData, err := contracts.NewCallData(contracts.NameFaucet, "withdrawTo", contractAddress, amount.ToBig())
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendExternalMessage(callData, types.FaucetAddress, nil)
}

func (c *DirectClient) Call(args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error) {
	blockNrOrHash, err := transport.AsBlockReference(blockId)
	if err != nil {
		return nil, err
	}
	return c.ethApi.Call(c.ctx, *args, transport.BlockNumberOrHash(blockNrOrHash), stateOverride)
}

func (c *DirectClient) RawCall(method string, params ...any) (json.RawMessage, error) {
	panic("Not supported")
}

func (c *DirectClient) CurrencyCreate(contractAddr types.Address, amount types.Value, name string, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "createToken", amount.ToBig(), name, withdraw)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
}

func (c *DirectClient) CurrencyWithdraw(contractAddr types.Address, amount types.Value, toAddr types.Address, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "withdrawToken", amount.ToBig(), toAddr)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
}

func (c *DirectClient) CurrencyMint(contractAddr types.Address, amount types.Value, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error) {
	data, err := contracts.NewCallData(contracts.NameNilCurrencyBase, "mintToken", amount.ToBig(), withdraw)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(data, contractAddr, pk)
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
