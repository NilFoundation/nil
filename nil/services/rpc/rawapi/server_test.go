package rawapi

import (
	"context"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type testApi struct {
	handler func(types.ShardId) (ssz.SSZEncodedData, error)
}

func (t *testApi) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return t.handler(shardId)
}

type testNetworkTransportProtocol interface {
	TestMethod(pb.BlockRequest) pb.RawBlockResponse
}

type ApiServerTestSuite struct {
	suite.Suite

	ctx                  context.Context
	logger               zerolog.Logger
	serverNetworkManager *network.Manager
	clientNetworkManager *network.Manager
	serverPeerId         network.PeerID
}

func (s *ApiServerTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.logger = logging.NewLogger("Test")
}

func (s *ApiServerTestSuite) SetupTest() {
	networkManagers := network.NewTestManagers(s.T(), s.ctx, 2)
	s.clientNetworkManager = networkManagers[0]
	s.serverNetworkManager = networkManagers[1]
	_, s.serverPeerId = network.ConnectManagers(s.T(), s.clientNetworkManager, s.serverNetworkManager)
}

type ApiWithoutTestMethod struct{}

type ApiWithWrongMethodArguments struct{}

func (api *ApiWithWrongMethodArguments) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference, extraArg int) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type ApiWithWrongContextMethodArgument struct{}

func (api *ApiWithWrongContextMethodArgument) TestMethod(notContextArgument int, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type ApiWithWrongMethodReturn struct{}

func (api *ApiWithWrongMethodReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (int, error) {
	return 0, nil
}

type ApiWithPointerInsteadOfValueMethodReturn struct{}

func (api *ApiWithPointerInsteadOfValueMethodReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*ssz.SSZEncodedData, error) {
	return &ssz.SSZEncodedData{}, nil
}

type ApiWithWrongErrorTypeReturn struct{}

func (api *ApiWithWrongErrorTypeReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, int) {
	return nil, 0
}

func (s *ApiServerTestSuite) TestWrongApis() {
	s.T().Parallel()

	protocolInterfaceType := reflect.TypeOf((*testNetworkTransportProtocol)(nil)).Elem()

	wrongApis := []interface{}{
		&ApiWithoutTestMethod{},
		&ApiWithWrongMethodArguments{},
		&ApiWithWrongContextMethodArgument{},
		&ApiWithWrongMethodReturn{},
		&ApiWithPointerInsteadOfValueMethodReturn{},
		&ApiWithWrongErrorTypeReturn{},
	}

	for _, a := range wrongApis {
		api := a
		s.Run(reflect.TypeOf(api).String(), func() {
			err := setRawApiRequestHandlers(s.ctx, protocolInterfaceType, api, "testapi", s.serverNetworkManager, s.logger)
			s.Require().ErrorIs(err, ErrRequestHandlerCreation)
		})
	}
}

func TestApiServer(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApiServerTestSuite))
}

type ApiServerResponsesTestSuite struct {
	ApiServerTestSuite
	api *testApi
}

func (s *ApiServerResponsesTestSuite) SetupTest() {
	s.ApiServerTestSuite.SetupTest()

	protocolInterfaceType := reflect.TypeOf((*testNetworkTransportProtocol)(nil)).Elem()
	s.api = &testApi{}
	err := setRawApiRequestHandlers(s.ctx, protocolInterfaceType, s.api, "testapi", s.serverNetworkManager, s.logger)
	s.Require().NoError(err)
}

func (s *ApiServerResponsesTestSuite) makeValidLatestBlockRequest(shardId types.ShardId) []byte {
	s.T().Helper()

	request := &pb.BlockRequest{
		ShardId: uint32(shardId),
		Reference: &pb.BlockReference{
			Reference: &pb.BlockReference_NamedBlockReference{
				NamedBlockReference: pb.NamedBlockReference_LatestBlock,
			},
		},
	}
	requestBytes, err := proto.Marshal(request)
	s.Require().NoError(err)
	return requestBytes
}

func (s *ApiServerResponsesTestSuite) makeInvalidBlockRequest() []byte {
	s.T().Helper()

	request := &pb.BlockRequest{
		ShardId:   123,
		Reference: &pb.BlockReference{}, // No oneof field option selected
	}
	requestBytes, err := proto.Marshal(request)
	s.Require().NoError(err)
	return requestBytes
}

func (s *ApiServerResponsesTestSuite) TestValidResponse() {
	lastCallShardId := new(types.ShardId)
	s.api.handler = func(shardId types.ShardId) (ssz.SSZEncodedData, error) {
		*lastCallShardId = shardId
		return (shardId * 2).Bytes(), nil
	}

	request := s.makeValidLatestBlockRequest(123)
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(s.ctx, s.serverPeerId, "testapi/TestMethod", request)
	s.Require().NoError(err)
	s.Require().Equal(types.ShardId(123), *lastCallShardId)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().EqualValues(246, types.BytesToShardId(pbResponse.GetData().BlockSSZ))
}

func (s *ApiServerResponsesTestSuite) TestNilResponse() {
	s.api.handler = func(shardId types.ShardId) (ssz.SSZEncodedData, error) {
		return nil, nil
	}

	request := s.makeValidLatestBlockRequest(123)
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(s.ctx, s.serverPeerId, "testapi/TestMethod", request)
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Equal("block should not be nil", pbResponse.GetError().Message)
}

func (s *ApiServerResponsesTestSuite) TestInvalidSchemaRequest() {
	s.api.handler = func(shardId types.ShardId) (ssz.SSZEncodedData, error) {
		return ssz.SSZEncodedData{}, nil
	}

	response, err := s.clientNetworkManager.SendRequestAndGetResponse(s.ctx, s.serverPeerId, "testapi/TestMethod", []byte("invalid request"))
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Contains(pbResponse.GetError().Message, "cannot parse invalid wire-format data")
}

func (s *ApiServerResponsesTestSuite) TestInvalidDataRequest() {
	s.api.handler = func(shardId types.ShardId) (ssz.SSZEncodedData, error) {
		return ssz.SSZEncodedData{}, nil
	}

	request := s.makeInvalidBlockRequest()
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(s.ctx, s.serverPeerId, "testapi/TestMethod", request)
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Equal("unexpected block reference type", pbResponse.GetError().Message)
}

func (s *ApiServerResponsesTestSuite) TestHandlerPanic() {
	s.api.handler = func(shardId types.ShardId) (ssz.SSZEncodedData, error) {
		panic("test panic")
	}

	request := s.makeValidLatestBlockRequest(123)
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(s.ctx, s.serverPeerId, "testapi/TestMethod", request)
	s.Require().NoError(err)

	s.Require().Empty(response)
}

func TestApiServerResponses(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApiServerResponsesTestSuite))
}
