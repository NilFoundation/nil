package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*HexedDebugRPCBlock, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*HexedDebugRPCBlock, error)
	GetContract(ctx context.Context, contractAddr types.Address, blockNrOrHash transport.BlockNumberOrHash) (*DebugRPCContract, error)
}

type DebugAPIImpl struct {
	*BaseAPI
	db       db.ReadOnlyDB
	logger   zerolog.Logger
	accessor *execution.StateAccessor
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(base *BaseAPI, db db.ReadOnlyDB, logger zerolog.Logger) *DebugAPIImpl {
	accessor, _ := execution.NewStateAccessor()
	return &DebugAPIImpl{
		BaseAPI:  base,
		db:       db,
		logger:   logger,
		accessor: accessor,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*HexedDebugRPCBlock, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	if number == transport.LatestBlockNumber {
		hash, err := db.ReadLastBlockHash(tx, shardId)
		if err != nil {
			return nil, err
		}

		return api.getBlockByHash(tx, shardId, hash, withMessages)
	}

	blockHash, err := db.ReadBlockHashByNumber(tx, shardId, number.BlockNumber())
	if err != nil {
		return nil, err
	}
	return api.getBlockByHash(tx, shardId, blockHash, withMessages)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*HexedDebugRPCBlock, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	return api.getBlockByHash(tx, shardId, hash, withMessages)
}

func (api *DebugAPIImpl) getBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash, withMessages bool) (*HexedDebugRPCBlock, error) {
	accessor := api.accessor.Access(tx, shardId).GetBlock()
	if withMessages {
		accessor = accessor.WithInMessages().WithOutMessages().WithReceipts()
	}

	data, err := accessor.ByHash(hash)
	if err != nil {
		return nil, err
	}

	if data.Block() == nil {
		return nil, nil
	}

	blockHash := data.Block().Hash()
	if blockHash != hash {
		return nil, fmt.Errorf("block hash mismatch: expected %x, got %x", hash, blockHash)
	}

	block := &types.BlockWithExtractedData{
		Block: data.Block(),
	}

	if withMessages {
		// TODO: StateAccessor without decoding
		block.InMessages = data.InMessages()
		block.OutMessages = data.OutMessages()
		block.Receipts = data.Receipts()
		block.Errors = make(map[common.Hash]string)
		for _, message := range block.InMessages {
			msgHash := message.Hash()
			errMsg, err := db.ReadError(tx, msgHash)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			if len(errMsg) > 0 {
				block.Errors[msgHash] = errMsg
			}
		}
	}
	b, err := block.EncodeSSZ()
	if err != nil {
		return nil, err
	}
	return EncodeBlockWithRawExtractedData(b)
}

func (api *DebugAPIImpl) GetContract(ctx context.Context, contractAddr types.Address, blockNrOrHash transport.BlockNumberOrHash) (*DebugRPCContract, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	shardId := contractAddr.ShardId()

	blockHash, err := extractBlockHash(tx, shardId, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	accessor := api.accessor.Access(tx, shardId).GetBlock()
	data, err := accessor.ByHash(blockHash)
	if err != nil {
		return nil, err
	}

	if data.Block() == nil {
		return nil, nil
	}

	contractRawReader := mpt.NewDbReader(tx, shardId, db.ContractTrieTable)
	contractRawReader.SetRootHash(data.Block().SmartContractsRoot)
	contractRaw, err := contractRawReader.Get(contractAddr.Hash().Bytes())
	if err != nil {
		return nil, err
	}
	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(contractRaw); err != nil {
		return nil, err
	}

	storageReader := execution.NewDbStorageTrieReader(tx, shardId)
	storageReader.SetRootHash(contract.StorageRoot)
	entries, err := storageReader.Entries()
	if err != nil {
		return nil, err
	}

	code, err := db.ReadCode(tx, shardId, contract.CodeHash)
	if err != nil {
		return nil, err
	}

	proof, err := contractRawReader.CreateProof(contractAddr.Hash().Bytes())
	if err != nil {
		return nil, err
	}

	hexed := make([]hexutil.Bytes, len(proof))
	for i, val := range proof {
		// Mb make proof follow fastssz.Marshaler interface to use hexify func below?
		valBytes, err := val.Encode()
		if err != nil {
			return nil, err
		}
		hexed[i] = valBytes
	}

	return &DebugRPCContract{Code: hexutil.Bytes(code), Contract: contractRaw, Proof: hexed, Storage: entries}, nil
}
