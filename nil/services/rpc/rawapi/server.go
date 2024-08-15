package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"google.golang.org/protobuf/proto"
)

func SetRawApiRequestHandler(ctx context.Context, api Api, manager *network.Manager) {
	// TODO: use reflection to set handler per method (+ figure out how to match query types and extract values from them to call methods)
	manager.SetRequestHandler(ctx, "get_block_header", func(ctx context.Context, request []byte) ([]byte, error) {
		var blockRequestPb pb.BlockRequest
		if err := proto.Unmarshal(request, &blockRequestPb); err != nil {
			return proto.Marshal(toPbRawBlockResponseError(err))
		}
		blockReference, shardId := blockReferenceAndShardIdFromPbBlockRequest(&blockRequestPb)

		block, err := api.GetBlockHeader(ctx, shardId, blockReference)
		if err != nil {
			return proto.Marshal(toPbRawBlockResponseError(err))
		}
		return proto.Marshal(toPbRawBlockResponse(&types.BlockWithRawExtractedData{Block: block}))
	})

	manager.SetRequestHandler(ctx, "get_full_block_data", func(ctx context.Context, request []byte) ([]byte, error) {
		var blockRequestPb pb.BlockRequest
		if err := proto.Unmarshal(request, &blockRequestPb); err != nil {
			return proto.Marshal(toPbRawBlockResponseError(err))
		}
		blockReference, shardId := blockReferenceAndShardIdFromPbBlockRequest(&blockRequestPb)

		block, err := api.GetFullBlockData(ctx, shardId, blockReference)
		if err != nil {
			return proto.Marshal(toPbRawBlockResponseError(err))
		}
		return proto.Marshal(toPbRawBlockResponse(block))
	})
}

func toPbRawBlock(block *types.BlockWithRawExtractedData) (*pb.RawBlock, error) {
	blockSSZ, err := block.Block.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return &pb.RawBlock{
		BlockSSZ:       blockSSZ,
		InMessagesSSZ:  block.InMessages,
		OutMessagesSSZ: block.OutMessages,
		ReceiptsSSZ:    block.Receipts,
		Errors:         toPbErrorMap(block.Errors),
	}, nil
}

func toPbRawBlockResponse(block *types.BlockWithRawExtractedData) *pb.RawBlockResponse {
	data, err := toPbRawBlock(block)
	if err != nil {
		return toPbRawBlockResponseError(err)
	}

	return &pb.RawBlockResponse{
		Result: &pb.RawBlockResponse_Data{
			Data: data,
		},
	}
}

func toPbRawBlockResponseError(err error) *pb.RawBlockResponse {
	return &pb.RawBlockResponse{
		Result: &pb.RawBlockResponse_Error{
			Error: toPbError(err),
		},
	}
}
