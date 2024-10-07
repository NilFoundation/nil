package rawapi

import (
	"context"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type generatedApiClient struct {
	apiCodec       apiCodec
	networkManager *network.Manager
	serverPeerId   network.PeerID
}

type ApiClientTestSuite struct {
	RawApiTestSuite

	apiClient *generatedApiClient
}

func newGeneratedApiClient(networkManager *network.Manager, serverPeerId network.PeerID) (*generatedApiClient, error) {
	apiCodec, err := newApiCodec(reflect.TypeOf(&generatedApiClient{}), reflect.TypeFor[compatibleNetworkTransportProtocol]())
	if err != nil {
		return nil, err
	}
	return &generatedApiClient{
		apiCodec:       apiCodec,
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
	}, nil
}

func (api *generatedApiClient) TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return sendRequestAndGetResponse[ssz.SSZEncodedData](api.apiCodec, api.networkManager, api.serverPeerId, types.BaseShardId, "testapi", "TestMethod", ctx, blockReference)
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

func (s *ApiClientTestSuite) doRequest() (ssz.SSZEncodedData, error) {
	return s.apiClient.TestMethod(s.ctx, rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
}

func (s *ApiClientTestSuite) TestValidResponse() {
	var index types.MessageIndex
	s.serverNetworkManager.SetRequestHandler(s.ctx, "/shard/1/testapi/TestMethod", func(ctx context.Context, request []byte) ([]byte, error) {
		var blockRequest pb.BlockRequest
		s.Require().NoError(proto.Unmarshal(request, &blockRequest))

		index += 1
		response := &pb.RawBlockResponse{
			Result: &pb.RawBlockResponse_Data{
				Data: &pb.RawBlock{
					BlockSSZ: index.Bytes(),
				},
			},
		}
		index += 1
		resp, err := proto.Marshal(response)
		return resp, err
	})

	response, err := s.doRequest()
	s.Require().NoError(err)
	s.Require().EqualValues(2, index)
	s.Require().EqualValues(1, types.BytesToMessageIndex(response))
}

func (s *ApiClientTestSuite) TestInvalidResponse() {
	requestHandlerCalled := new(bool)
	s.serverNetworkManager.SetRequestHandler(s.ctx, "/shard/1/testapi/TestMethod", func(ctx context.Context, request []byte) ([]byte, error) {
		*requestHandlerCalled = true
		return nil, nil
	})

	_, err := s.doRequest()
	s.Require().ErrorContains(err, "unexpected response")
}

func (s *ApiClientTestSuite) TestErrorResponse() {
	requestHandlerCalled := new(bool)
	s.serverNetworkManager.SetRequestHandler(s.ctx, "/shard/1/testapi/TestMethod", func(ctx context.Context, request []byte) ([]byte, error) {
		*requestHandlerCalled = true
		response := &pb.RawBlockResponse{
			Result: &pb.RawBlockResponse_Error{
				Error: &pb.Error{
					Message: "Test error",
				},
			},
		}
		resp, err := proto.Marshal(response)
		return resp, err
	})

	_, err := s.doRequest()
	s.Require().ErrorContains(err, "Test error")
}

func TestClient(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApiClientTestSuite))
}
