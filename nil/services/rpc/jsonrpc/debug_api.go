package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*HexedDebugRPCBlock, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*HexedDebugRPCBlock, error)
	GetContract(ctx context.Context, contractAddr types.Address, blockNrOrHash transport.BlockNumberOrHash) (*DebugRPCContract, error)
}

type DebugAPIImpl struct {
	db       db.ReadOnlyDB
	logger   zerolog.Logger
	accessor *execution.StateAccessor
	rawApi   rawapi.Api
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(rawApi rawapi.Api, db db.ReadOnlyDB, logger zerolog.Logger) *DebugAPIImpl {
	accessor, _ := execution.NewStateAccessor()
	return &DebugAPIImpl{
		db:       db,
		logger:   logger,
		accessor: accessor,
		rawApi:   rawApi,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*HexedDebugRPCBlock, error) {
	var blockReference rawapi.BlockReference
	if number <= 0 {
		switch number {
		case transport.LatestBlockNumber:
			blockReference = rawapi.NamedBlockIdentifierAsBlockReference(rawapi.LatestBlock)
		case transport.EarliestBlockNumber:
			blockReference = rawapi.NamedBlockIdentifierAsBlockReference(rawapi.EarliestBlock)
		case transport.LatestExecutedBlockNumber:
		case transport.FinalizedBlockNumber:
		case transport.SafeBlockNumber:
		case transport.PendingBlockNumber:
		default:
			return nil, fmt.Errorf("not supported special block number %s", number)
		}
	} else {
		blockReference = rawapi.BlockNumberAsBlockReference(types.BlockNumber(number))
	}
	return api.getBlockByReference(ctx, shardId, blockReference, withMessages)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*HexedDebugRPCBlock, error) {
	return api.getBlockByReference(ctx, shardId, rawapi.BlockHashAsBlockReference(hash), withMessages)
}

func (api *DebugAPIImpl) getBlockByReference(ctx context.Context, shardId types.ShardId, blockReference rawapi.BlockReference, withMessages bool) (*HexedDebugRPCBlock, error) {
	var blockData *types.BlockWithRawExtractedData
	var err error
	if withMessages {
		blockData, err = api.rawApi.GetFullBlockData(ctx, shardId, blockReference)
		if err != nil {
			return nil, err
		}
	} else {
		blockHeader, err := api.rawApi.GetBlockHeader(ctx, shardId, blockReference)
		if err != nil {
			return nil, err
		}
		blockData = &types.BlockWithRawExtractedData{Block: blockHeader}
	}
	return EncodeBlockWithRawExtractedData(blockData)
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

	proof, err := mpt.BuildProof(contractRawReader, contractAddr.Hash().Bytes(), mpt.ReadMPTOperation)
	if err != nil {
		return nil, err
	}

	encodedProof, err := proof.Encode()
	if err != nil {
		return nil, err
	}

	return &DebugRPCContract{Code: hexutil.Bytes(code), Contract: contractRaw, Proof: hexutil.Bytes(encodedProof), Storage: entries}, nil
}
