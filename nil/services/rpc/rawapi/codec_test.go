package rawapi

import (
	"context"
	"reflect"
	"testing"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/stretchr/testify/require"
)

type compatibleNetworkTransportProtocol interface {
	TestMethod(pb.BlockRequest) pb.RawBlockResponse
}

type compatibleApi interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
}

type apiWithOtherMethod interface {
	OtherMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
}

type apiWithWrongMethodArguments interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference, extraArg int) (ssz.SSZEncodedData, error)
}

type apiWithWrongContextMethodArgument interface {
	TestMethod(notContextArgument int, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
}

type apiWithWrongMethodReturn interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (int, error)
}

type apiWithPointerInsteadOfValueMethodReturn interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (*ssz.SSZEncodedData, error)
}

type apiWithWrongErrorTypeReturn interface {
	TestMethod(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, int)
}

func TestApisCompatibility(t *testing.T) {
	t.Parallel()

	protocolInterfaceType := reflect.TypeFor[compatibleNetworkTransportProtocol]()

	incompatibleApis := map[reflect.Type]string{
		reflect.TypeFor[apiWithOtherMethod]():                       "method OtherMethod not found in rawapi.compatibleNetworkTransportProtocol",
		reflect.TypeFor[apiWithWrongMethodArguments]():              "API method TestMethod requires 2 arguments, but pb.BlockRequest.PackProtoMessage accepts 1 arguments",
		reflect.TypeFor[apiWithWrongContextMethodArgument]():        "first argument of API method TestMethod must be context.Context",
		reflect.TypeFor[apiWithWrongMethodReturn]():                 "API method outputs int type, but PackProtoMessage expects []uint8",
		reflect.TypeFor[apiWithPointerInsteadOfValueMethodReturn](): "API method outputs *[]uint8 type, but PackProtoMessage expects []uint8",
		reflect.TypeFor[apiWithWrongErrorTypeReturn]():              "second output argument of API method TestMethod must be error",
	}

	goodApiType := reflect.TypeFor[compatibleApi]()
	t.Run(goodApiType.String(), func(t *testing.T) {
		t.Parallel()
		_, err := newApiCodec(goodApiType, protocolInterfaceType)
		require.NoError(t, err)
	})

	for a, e := range incompatibleApis {
		api := a
		errStr := e
		t.Run(api.String(), func(t *testing.T) {
			t.Parallel()

			_, err := newApiCodec(api, protocolInterfaceType)
			require.ErrorContains(t, err, errStr)
		})
	}
}

type noArgsNetworkTransportProtocol interface {
	TestMethod() pb.RawBlockResponse
}

type noArgsApi interface {
	TestMethod(ctx context.Context) (ssz.SSZEncodedData, error)
}

type noArgsApiWithoutCtx interface {
	TestMethod() (ssz.SSZEncodedData, error)
}

func TestApisNoArgs(t *testing.T) {
	t.Parallel()

	protocolInterfaceType := reflect.TypeFor[noArgsNetworkTransportProtocol]()
	goodApiType := reflect.TypeFor[noArgsApi]()
	t.Run(goodApiType.String(), func(t *testing.T) {
		t.Parallel()

		_, err := newApiCodec(goodApiType, protocolInterfaceType)
		require.NoError(t, err)
	})

	incompatibleApis := map[reflect.Type]string{
		reflect.TypeFor[noArgsApiWithoutCtx](): "API method TestMethod must have at least one argument",
	}

	for a, e := range incompatibleApis {
		api := a
		errStr := e
		t.Run(reflect.TypeOf(api).String(), func(t *testing.T) {
			t.Parallel()

			_, err := newApiCodec(api, protocolInterfaceType)
			require.ErrorContains(t, err, errStr)
		})
	}
}
