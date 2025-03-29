package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
)

// Web3API provides interfaces for the web3_ RPC commands
type Web3API interface {
	ClientVersion(_ context.Context) (string, error)
}

type Web3APIImpl struct {
	rawApi rawapi.NodeApi
}

var _ Web3API = &Web3APIImpl{}

func NewWeb3API(rawApi rawapi.NodeApi) *Web3APIImpl {
	return &Web3APIImpl{
		rawApi: rawApi,
	}
}

// ClientVersion implements web3_clientVersion. Returns the current client version.
func (api *Web3APIImpl) ClientVersion(ctx context.Context) (string, error) {
	return api.rawApi.ClientVersion(ctx)
}
