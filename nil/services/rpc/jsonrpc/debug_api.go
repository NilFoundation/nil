package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type DebugAPI interface {
	GetBlockByNumber(
		ctx context.Context,
		shardId types.ShardId,
		number transport.BlockNumber,
		withTransactions bool,
	) (*DebugRPCBlock, error)
	GetBlockByHash(ctx context.Context, hash common.Hash, withTransactions bool) (*DebugRPCBlock, error)
	GetContract(
		ctx context.Context,
		contractAddr types.Address,
		blockNrOrHash transport.BlockNumberOrHash,
	) (*DebugRPCContract, error)
	GetBootstrapConfig(ctx context.Context) (*rpctypes.BootstrapConfig, error)
}

type DebugAPIImpl struct {
	logger logging.Logger
	rawApi rawapi.NodeApi
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(rawApi rawapi.NodeApi, logger logging.Logger) *DebugAPIImpl {
	return &DebugAPIImpl{
		logger: logger,
		rawApi: rawApi,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(
	ctx context.Context,
	shardId types.ShardId,
	number transport.BlockNumber,
	withTransactions bool,
) (*DebugRPCBlock, error) {
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
	return api.getBlockByReference(ctx, shardId, blockReference, withTransactions)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withTransactions bool,
) (*DebugRPCBlock, error) {
	shardId := types.ShardIdFromHash(hash)
	return api.getBlockByReference(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash), withTransactions)
}

func (api *DebugAPIImpl) getBlockByReference(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
	withTransactions bool,
) (*DebugRPCBlock, error) {
	var blockData *types.RawBlockWithExtractedData
	var err error
	if withTransactions {
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

func (api *DebugAPIImpl) GetContract(
	ctx context.Context,
	contractAddr types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (*DebugRPCContract, error) {
	contract, err := api.rawApi.GetContract(ctx, contractAddr, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}

	return &DebugRPCContract{
		Contract:     contract.ContractSSZ,
		Code:         hexutil.Bytes(contract.Code),
		Proof:        contract.ProofEncoded,
		Storage:      contract.Storage,
		Tokens:       contract.Tokens,
		AsyncContext: contract.AsyncContext,
	}, nil
}

func (api *DebugAPIImpl) GetBootstrapConfig(ctx context.Context) (*rpctypes.BootstrapConfig, error) {
	return api.rawApi.GetBootstrapConfig(ctx)
}
