package rawapi

import (
	"context"
	"fmt"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type shardApiRequestPerformerNetwork struct {
	shard          types.ShardId
	apiName        string
	networkManager network.Manager
	codec          apiCodec
}

var _ shardApiRequestPerformer = (*shardApiRequestPerformerNetwork)(nil)

func (api *shardApiRequestPerformerNetwork) doApiRequest(
	ctx context.Context, codec *methodCodec, args ...any,
) ([]byte, error) {
	return doNetworkShardApiRequest(ctx, api.networkManager, api.shard, api.apiName, codec, args...)
}

func (api *shardApiRequestPerformerNetwork) shardId() types.ShardId {
	return api.shard
}

func (api *shardApiRequestPerformerNetwork) setNodeApi(_ NodeApi) {}

func (api *shardApiRequestPerformerNetwork) setAsP2pRequestHandlersIfAllowed(
	_ context.Context,
	_ network.Manager,
	_ logging.Logger,
) error {
	return nil
}

func (api *shardApiRequestPerformerNetwork) apiCodec() apiCodec {
	return api.codec
}

func newShardApiClientNetwork[
	T interface {
		~*S
		shardApiRequestPerformerSetter
	},
	S any,
](
	shardId types.ShardId,
	apiName string,
	networkManager network.Manager,
	apiType reflect.Type,
	transportType reflect.Type,
) (T, error) {
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	var rv T = new(S)
	rv.setShardApiRequestPerformer(&shardApiRequestPerformerNetwork{
		shard:          shardId,
		apiName:        apiName,
		networkManager: networkManager,
		codec:          codec,
	})
	return rv, nil
}

func doNetworkShardApiRequest(
	ctx context.Context,
	networkManager network.Manager,
	shardId types.ShardId,
	apiName string,
	codec *methodCodec,
	args ...any,
) ([]byte, error) {
	protocol := network.ProtocolID(fmt.Sprintf("/shard/%d/%s/%s", shardId, apiName, codec.methodName))
	serverPeerId, err := discoverAppropriatePeer(networkManager, shardId, protocol)
	if err != nil {
		return nil, err
	}

	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return nil, err
	}

	return networkManager.SendRequestAndGetResponse(ctx, serverPeerId, protocol, requestBody)
}

func discoverAppropriatePeer(
	networkManager network.Manager,
	shardId types.ShardId,
	protocol network.ProtocolID,
) (network.PeerID, error) {
	peersWithSpecifiedShard := networkManager.GetPeersForProtocol(protocol)
	if len(peersWithSpecifiedShard) == 0 {
		return "", fmt.Errorf("no peers with shard %d found", shardId)
	}
	return peersWithSpecifiedShard[0], nil
}
