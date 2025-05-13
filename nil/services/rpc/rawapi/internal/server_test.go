package internal

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

var initialTcpPort atomic.Int32

type RawApiTestSuite struct {
	suite.Suite

	ctx                  context.Context
	logger               logging.Logger
	serverNetworkManager network.Manager
	clientNetworkManager network.Manager
	serverPeerId         network.PeerID
}

func (s *RawApiTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.logger = logging.NewLogger("Test")
	initialTcpPort.CompareAndSwap(0, 9010)
}

func (s *RawApiTestSuite) SetupTest() {
	networkManagers := network.NewTestManagers(s.ctx, s.T(), int(initialTcpPort.Add(2)), 2)
	s.clientNetworkManager = networkManagers[0]
	s.serverNetworkManager = networkManagers[1]
	_, s.serverPeerId = network.ConnectManagers(s.T(), s.clientNetworkManager, s.serverNetworkManager)
}

type testApiIface interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (serialization.EncodedData, error)
}

type testApi struct {
	handler func() (serialization.EncodedData, error)
}

func (t *testApi) TestMethod(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (serialization.EncodedData, error) {
	return t.handler()
}

type testNetworkTransportProtocol interface {
	TestMethod(pb.BlockRequest) pb.RawBlockResponse
}

type ApiServerTestSuite struct {
	RawApiTestSuite

	api *testApi
}

func (s *ApiServerTestSuite) SetupSuite() {
	s.RawApiTestSuite.SetupSuite()
}

func (s *ApiServerTestSuite) SetupTest() {
	s.RawApiTestSuite.SetupTest()

	protocolInterfaceType := reflect.TypeFor[testNetworkTransportProtocol]()
	apiInterfaceType := reflect.TypeFor[testApiIface]()
	s.api = &testApi{}
	err := setRawApiRequestHandlers(
		s.ctx,
		protocolInterfaceType,
		apiInterfaceType,
		s.api,
		types.BaseShardId,
		"testapi",
		s.serverNetworkManager,
		s.logger)
	s.Require().NoError(err)
}

func (s *ApiServerTestSuite) makeValidLatestBlockRequest() []byte {
	s.T().Helper()

	request := &pb.BlockRequest{
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

func (s *ApiServerTestSuite) makeInvalidBlockRequest() []byte {
	s.T().Helper()

	request := &pb.BlockRequest{
		Reference: &pb.BlockReference{}, // No oneof field option selected
	}
	requestBytes, err := proto.Marshal(request)
	s.Require().NoError(err)
	return requestBytes
}

func (s *ApiServerTestSuite) TestValidResponse() {
	var index types.TransactionIndex
	s.api.handler = func() (serialization.EncodedData, error) {
		index++
		return index.Bytes(), nil
	}

	request := s.makeValidLatestBlockRequest()
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(
		s.ctx, s.serverPeerId, "/shard/1/testapi/TestMethod", request)
	s.Require().NoError(err)
	s.Require().EqualValues(1, index)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().EqualValues(1, types.BytesToTransactionIndex(pbResponse.GetData().GetBlockBytes()))
}

func (s *ApiServerTestSuite) TestNilResponse() {
	s.api.handler = func() (serialization.EncodedData, error) {
		return nil, nil
	}

	request := s.makeValidLatestBlockRequest()
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(
		s.ctx, s.serverPeerId, "/shard/1/testapi/TestMethod", request)
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Equal("block should not be nil", pbResponse.GetError().GetMessage())
}

func (s *ApiServerTestSuite) TestInvalidSchemaRequest() {
	s.api.handler = func() (serialization.EncodedData, error) {
		return serialization.EncodedData{}, nil
	}

	response, err := s.clientNetworkManager.SendRequestAndGetResponse(
		s.ctx,
		s.serverPeerId,
		"/shard/1/testapi/TestMethod",
		[]byte("invalid request"))
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Contains(pbResponse.GetError().GetMessage(), "cannot parse invalid wire-format data")
}

func (s *ApiServerTestSuite) TestInvalidDataRequest() {
	s.api.handler = func() (serialization.EncodedData, error) {
		return serialization.EncodedData{}, nil
	}

	request := s.makeInvalidBlockRequest()
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(
		s.ctx, s.serverPeerId, "/shard/1/testapi/TestMethod", request)
	s.Require().NoError(err)

	var pbResponse pb.RawBlockResponse
	err = proto.Unmarshal(response, &pbResponse)
	s.Require().NoError(err)
	s.Require().NotNil(pbResponse.GetError())
	s.Require().Equal("unexpected block reference type", pbResponse.GetError().GetMessage())
}

func (s *ApiServerTestSuite) TestHandlerPanic() {
	s.api.handler = func() (serialization.EncodedData, error) {
		panic("test panic")
	}

	request := s.makeValidLatestBlockRequest()
	response, err := s.clientNetworkManager.SendRequestAndGetResponse(
		s.ctx, s.serverPeerId, "/shard/1/testapi/TestMethod", request)
	s.Require().NoError(err)

	s.Require().Empty(response)
}

func TestApiServerResponses(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ApiServerTestSuite))
}
