package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
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

func (api *NetworkRawApiAccessor) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	blockRequest := &pb.BlockRequest{}
	if err := blockRequest.PackProtoMessage(shardId, blockReference); err != nil {
		return nil, err
	}
	requestBody, err := proto.Marshal(blockRequest)
	if err != nil {
		return nil, err
	}

	responseBody, err := api.networkManager.SendRequestAndGetResponse(ctx, api.serverPeerId, "rawapi/GetBlockHeader", requestBody)
	if err != nil {
		return nil, err
	}

	var blockPb pb.RawBlockResponse
	err = proto.Unmarshal(responseBody, &blockPb)
	if err != nil {
		return nil, err
	}
	fullBlockData, err := blockPb.UnpackProtoMessage()
	if err != nil {
		return nil, err
	}
	return fullBlockData, nil
}

func (api *NetworkRawApiAccessor) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	blockRequest := &pb.BlockRequest{}
	if err := blockRequest.PackProtoMessage(shardId, blockReference); err != nil {
		return nil, err
	}
	requestBody, err := proto.Marshal(blockRequest)
	if err != nil {
		return nil, err
	}

	responseBody, err := api.networkManager.SendRequestAndGetResponse(ctx, api.serverPeerId, "rawapi/GetFullBlockData", requestBody)
	if err != nil {
		return nil, err
	}

	var blockPb pb.RawFullBlockResponse
	err = proto.Unmarshal(responseBody, &blockPb)
	if err != nil {
		return nil, err
	}
	return blockPb.UnpackProtoMessage()
}
