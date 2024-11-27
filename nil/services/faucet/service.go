package faucet

import (
	"context"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

type Service struct {
	client client.Client
}

func NewService(client client.Client) (*Service, error) {
	return &Service{client: client}, nil
}

func (s *Service) Run(ctx context.Context, endpoint string) error {
	err := s.startRpcServer(ctx, endpoint)
	return err
}

func (s *Service) startRpcServer(ctx context.Context, endpoint string) error {
	logger := logging.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
	}

	faucetApi := NewFaucetAPI(ctx, s.client, &logger)

	apiList := []transport.API{
		{
			Namespace: "faucet",
			Public:    true,
			Service:   FaucetAPI(faucetApi),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}
