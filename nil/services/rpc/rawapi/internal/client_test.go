package internal

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type generatedApiClientIface interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (sszx.SSZEncodedData, error)
}

type generatedApiClient struct {
	apiCodec       apiCodec
	networkManager network.Manager
	serverPeerId   network.PeerID
	doApiRequest   doApiRequestFunction
}

type ApiClientTestSuite struct {
	RawApiTestSuite

	apiClient *generatedApiClient
}

func newGeneratedApiClient(networkManager network.Manager, serverPeerId network.PeerID) (*generatedApiClient, error) {
	apiCodec, err := newApiCodec(
		reflect.TypeFor[generatedApiClientIface](),
		reflect.TypeFor[compatibleNetworkTransportProtocol]())
	if err != nil {
		return nil, err
	}
	return &generatedApiClient{
		apiCodec:       apiCodec,
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
		doApiRequest: func(ctx context.Context, codec *methodCodec, args ...any) ([]byte, error) {
			return doNetworkShardApiRequest(ctx, networkManager, types.BaseShardId, "testapi", codec, args...)
		},
	}, nil
}

func (api *generatedApiClient) TestMethod(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	return sendRequestAndGetResponse[sszx.SSZEncodedData](
		ctx, api.doApiRequest, api.apiCodec, "TestMethod", blockReference)
}

func (s *ApiClientTestSuite) SetupSuite() {
	s.RawApiTestSuite.SetupSuite()
}

func (s *ApiClientTestSuite) SetupTest() {
	s.RawApiTestSuite.SetupTest()

	var err error
	s.apiClient, err = newGeneratedApiClient(s.clientNetworkManager, s.serverPeerId)
	s.Require().NoError(err)
}

func (s *ApiClientTestSuite) doRequest() (sszx.SSZEncodedData, error) {
	return s.apiClient.TestMethod(s.ctx, rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
}

func (s *ApiClientTestSuite) waitForRequestHandler(protocol protocol.ID) {
	s.T().Helper()
	s.Eventually(
		func() bool {
			return len(s.clientNetworkManager.GetPeersForProtocol(protocol)) != 0
		},
		10*time.Second,
		100*time.Millisecond)
}

func (s *ApiClientTestSuite) TestValidResponse() {
	var index types.TransactionIndex
	s.serverNetworkManager.SetRequestHandler(
		s.ctx,
		"/shard/1/testapi/TestMethod",
		func(ctx context.Context, request []byte) ([]byte, error) {
			var blockRequest pb.BlockRequest
			s.Require().NoError(proto.Unmarshal(request, &blockRequest))

			index++
			response := &pb.RawBlockResponse{
				Result: &pb.RawBlockResponse_Data{
					Data: &pb.RawBlock{
						BlockSSZ: index.Bytes(),
					},
				},
			}
			index++
			resp, err := proto.Marshal(response)
			return resp, err
		})
	s.waitForRequestHandler("/shard/1/testapi/TestMethod")

	response, err := s.doRequest()
	s.Require().NoError(err)
	s.Require().EqualValues(2, index)
	s.Require().EqualValues(1, types.BytesToTransactionIndex(response))
}

func (s *ApiClientTestSuite) TestInvalidResponse() {
	requestHandlerCalled := new(bool)
	s.serverNetworkManager.SetRequestHandler(
		s.ctx,
		"/shard/1/testapi/TestMethod",
		func(ctx context.Context, request []byte) ([]byte, error) {
			*requestHandlerCalled = true
			return nil, nil
		})
	s.waitForRequestHandler("/shard/1/testapi/TestMethod")

	_, err := s.doRequest()
	s.Require().ErrorContains(err, "unexpected response")
}

func (s *ApiClientTestSuite) TestErrorResponse() {
	requestHandlerCalled := new(bool)
	s.serverNetworkManager.SetRequestHandler(
		s.ctx,
		"/shard/1/testapi/TestMethod",
		func(ctx context.Context, request []byte) ([]byte, error) {
			*requestHandlerCalled = true
			response := &pb.RawBlockResponse{
				Result: &pb.RawBlockResponse_Error{
					Error: &pb.Error{
						Message: "Test error",
					},
				},
			}
			return proto.Marshal(response)
		})
	s.waitForRequestHandler("/shard/1/testapi/TestMethod")

	_, err := s.doRequest()
	s.Require().ErrorContains(err, "Test error")
}

func TestClient(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApiClientTestSuite))
}
