package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*DebugRPCBlock, error)
	GetBlockByHash(ctx context.Context, hash common.Hash, withMessages bool) (*DebugRPCBlock, error)
	GetContract(ctx context.Context, contractAddr types.Address, blockNrOrHash transport.BlockNumberOrHash) (*DebugRPCContract, error)
}

type DebugAPIImpl struct {
	db       db.ReadOnlyDB
	logger   zerolog.Logger
	accessor *execution.StateAccessor
	rawApi   rawapi.NodeApi
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(rawApi rawapi.NodeApi, db db.ReadOnlyDB, logger zerolog.Logger) *DebugAPIImpl {
	accessor := execution.NewStateAccessor()
	return &DebugAPIImpl{
		db:       db,
		logger:   logger,
		accessor: accessor,
		rawApi:   rawApi,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*DebugRPCBlock, error) {
	var blockReference rawapitypes.BlockReference
	if number <= 0 {
		switch number {
		case transport.LatestBlockNumber:
			blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock)
		case transport.EarliestBlockNumber:
			blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.EarliestBlock)
		case transport.LatestExecutedBlockNumber:
		case transport.FinalizedBlockNumber:
		case transport.SafeBlockNumber:
		case transport.PendingBlockNumber:
		default:
			return nil, fmt.Errorf("not supported special block number %s", number)
		}
	} else {
		blockReference = rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(number))
	}
	return api.getBlockByReference(ctx, shardId, blockReference, withMessages)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, hash common.Hash, withMessages bool) (*DebugRPCBlock, error) {
	shardId := types.ShardIdFromHash(hash)
	return api.getBlockByReference(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash), withMessages)
}

func (api *DebugAPIImpl) getBlockByReference(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference, withMessages bool) (*DebugRPCBlock, error) {
	var blockData *types.RawBlockWithExtractedData
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
		blockData = &types.RawBlockWithExtractedData{Block: blockHeader}
	}
	return EncodeRawBlockWithExtractedData(blockData)
}

func (api *DebugAPIImpl) GetContract(ctx context.Context, contractAddr types.Address, blockNrOrHash transport.BlockNumberOrHash) (*DebugRPCContract, error) {
	contract, err := api.rawApi.GetContract(ctx, contractAddr, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}

	return &DebugRPCContract{
		Contract: contract.ContractSSZ,
		Code:     hexutil.Bytes(contract.Code),
		Proof:    contract.ProofEncoded,
		Storage:  contract.Storage,
	}, nil
}
