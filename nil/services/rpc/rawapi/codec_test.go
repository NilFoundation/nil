package rawapi

import (
	"context"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/stretchr/testify/require"
)

type compatibleNetworkTransportProtocol interface {
	TestMethod(pb.BlockRequest) pb.RawBlockResponse
}

type compatibleApi struct{}

func (t *compatibleApi) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type apiWithOtherMethod struct{}

func (api *apiWithOtherMethod) OtherMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type apiWithWrongMethodArguments struct{}

func (api *apiWithWrongMethodArguments) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference, extraArg int) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type apiWithWrongContextMethodArgument struct{}

func (api *apiWithWrongContextMethodArgument) TestMethod(notContextArgument int, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return nil, nil
}

type apiWithWrongMethodReturn struct{}

func (api *apiWithWrongMethodReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (int, error) {
	return 0, nil
}

type apiWithPointerInsteadOfValueMethodReturn struct{}

func (api *apiWithPointerInsteadOfValueMethodReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*ssz.SSZEncodedData, error) {
	return &ssz.SSZEncodedData{}, nil
}

type apiWithWrongErrorTypeReturn struct{}

func (api *apiWithWrongErrorTypeReturn) TestMethod(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, int) {
	return nil, 0
}

func TestApisCompatibility(t *testing.T) {
	t.Parallel()

	protocolInterfaceType := reflect.TypeOf((*compatibleNetworkTransportProtocol)(nil)).Elem()

	incompatibleApis := map[interface{}]string{
		&apiWithOtherMethod{}:                       "method OtherMethod not found in rawapi.compatibleNetworkTransportProtocol",
		&apiWithWrongMethodArguments{}:              "API method TestMethod requires 3 arguments, but pb.BlockRequest.PackProtoMessage accepts 3 arguments",
		&apiWithWrongContextMethodArgument{}:        "first argument of API method TestMethod must be context.Context",
		&apiWithWrongMethodReturn{}:                 "API method outputs int type, but PackProtoMessage expects []uint8",
		&apiWithPointerInsteadOfValueMethodReturn{}: "API method outputs *[]uint8 type, but PackProtoMessage expects []uint8",
		&apiWithWrongErrorTypeReturn{}:              "second output argument of API method TestMethod must be error",
	}

	goodApiType := reflect.TypeOf(&compatibleApi{})
	t.Run(goodApiType.String(), func(t *testing.T) {
		_, err := newApiCodec(goodApiType, protocolInterfaceType)
		require.NoError(t, err)
	})

	for a, e := range incompatibleApis {
		api := a
		errStr := e
		t.Run(reflect.TypeOf(api).String(), func(t *testing.T) {
			t.Parallel()

			_, err := newApiCodec(reflect.TypeOf(api), protocolInterfaceType)
			require.ErrorContains(t, err, errStr)
		})
	}
}
