package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type blockDebugRpcClient struct {
	client client.RawClient
}

func NewBlockDebugRpcClient(apiEndpoint string, logger logging.Logger) public.BlockDebugApi {
	return &blockDebugRpcClient{
		client: NewRetryClient(apiEndpoint, logger),
	}
}

func (c blockDebugRpcClient) GetLatestFetched(ctx context.Context) (public.BlockRefs, error) {
	return doRPCCall[public.BlockRefs](
		ctx,
		c.client,
		public.DebugGetLatestFetched,
	)
}

func (c blockDebugRpcClient) GetStateRootData(ctx context.Context) (*public.StateRootData, error) {
	return doRPCCall[*public.StateRootData](
		ctx,
		c.client,
		public.DebugGetStateRootData,
	)
}

func (c blockDebugRpcClient) GetBatchView(
	ctx context.Context, batchId public.BatchId,
) (*public.BatchViewDetailed, error) {
	return doRPCCall2[public.BatchId, *public.BatchViewDetailed](
		ctx,
		c.client,
		public.DebugGetBatchView,
		batchId,
	)
}

func (c blockDebugRpcClient) GetBatchViews(
	ctx context.Context,
	request public.BatchDebugRequest,
) ([]*public.BatchViewCompact, error) {
	return doRPCCall2[public.BatchDebugRequest, []*public.BatchViewCompact](
		ctx,
		c.client,
		public.DebugGetBatchViews,
		request,
	)
}

func (c blockDebugRpcClient) GetBatchStats(ctx context.Context) (*public.BatchStats, error) {
	return doRPCCall[*public.BatchStats](
		ctx,
		c.client,
		public.DebugGetBatchStats,
	)
}
