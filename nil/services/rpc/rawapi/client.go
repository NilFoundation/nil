package rawapi

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"google.golang.org/protobuf/proto"
)

type NetworkRawApiAccessor struct {
	networkManager *network.Manager
	serverPeerId   network.PeerID
}

var _ Api = (*NetworkRawApiAccessor)(nil)

func NewNetworkRawApiAccessor(ctx context.Context, networkManager *network.Manager, serverAddress string) (*NetworkRawApiAccessor, error) {
	serverPeerId, err := networkManager.Connect(ctx, serverAddress)
	if err != nil {
		return nil, err
	}
	return &NetworkRawApiAccessor{
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
	}, nil
}

func (api *NetworkRawApiAccessor) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.Block, error) {
	requestBody, err := proto.Marshal(makePbBlockRequest(shardId, blockReference))
	if err != nil {
		return nil, err
	}

	responseBody, err := api.networkManager.SendRequestAndGetResponse(ctx, api.serverPeerId, "get_block_header", requestBody)
	if err != nil {
		return nil, err
	}

	var blockPb pb.RawBlockResponse
	err = proto.Unmarshal(responseBody, &blockPb)
	if err != nil {
		return nil, err
	}
	fullBlockData, err := fromPbRawBlockWithExtraDataResponse(&blockPb)
	if err != nil {
		return nil, err
	}
	return fullBlockData.Block, nil
}

func (api *NetworkRawApiAccessor) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.BlockWithRawExtractedData, error) {
	requestBody, err := proto.Marshal(makePbBlockRequest(shardId, blockReference))
	if err != nil {
		return nil, err
	}

	responseBody, err := api.networkManager.SendRequestAndGetResponse(ctx, api.serverPeerId, "get_full_block_data", requestBody)
	if err != nil {
		return nil, err
	}

	var blockPb pb.RawBlockResponse
	err = proto.Unmarshal(responseBody, &blockPb)
	if err != nil {
		return nil, err
	}
	return fromPbRawBlockWithExtraDataResponse(&blockPb)
}

func fromPbRawBlockWithExtraDataResponse(response *pb.RawBlockResponse) (*types.BlockWithRawExtractedData, error) {
	switch response.Result.(type) {
	case *pb.RawBlockResponse_Error:
		return nil, fromPbError(response.GetError())
	case *pb.RawBlockResponse_Data:
		return fromPbRawBlockWithExtraData(response.GetData())
	default:
		return nil, errors.New("unexpected response")
	}
}

func fromPbRawBlockWithExtraData(pbBlock *pb.RawBlock) (*types.BlockWithRawExtractedData, error) {
	var block types.Block
	if err := block.UnmarshalSSZ(pbBlock.BlockSSZ); err != nil {
		return nil, err
	}
	return &types.BlockWithRawExtractedData{
		Block:       &block,
		InMessages:  pbBlock.InMessagesSSZ,
		OutMessages: pbBlock.OutMessagesSSZ,
		Receipts:    pbBlock.ReceiptsSSZ,
		Errors:      fromPbErrorMap(pbBlock.Errors),
	}, nil
}
