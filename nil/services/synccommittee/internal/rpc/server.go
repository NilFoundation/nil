package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type ServerConfig struct {
	Endpoint string
}

func NewServerConfig(endpoint string) ServerConfig {
	return ServerConfig{
		Endpoint: endpoint,
	}
}

type Handler struct {
	Namespace string
	Service   any
}

func NewHandler(namespace string, service any) Handler {
	return Handler{
		Namespace: namespace,
		Service:   service,
	}
}

type server struct {
	logger   logging.Logger
	config   ServerConfig
	handlers []Handler
}

func NewServer(
	config ServerConfig,
	logger logging.Logger,
	handlers ...Handler,
) *server {
	return &server{
		logger:   logger,
		config:   config,
		handlers: handlers,
	}
}

func (s *server) Name() string {
	return "rpc_server"
}

func (s *server) Run(context context.Context, started chan<- struct{}) error {
	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         s.config.Endpoint,
		HttpCompression: true,
		TraceRequests:   s.logger.GetLevel() < zerolog.InfoLevel, // debug or trace
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	apiList := make([]transport.API, 0, len(s.handlers))
	for _, handler := range s.handlers {
		apiList = append(apiList, transport.API{
			Namespace: handler.Namespace,
			Public:    true,
			Service:   handler.Service,
			Version:   "1.0",
		})
	}

	return rpc.StartRpcServer(context, httpConfig, apiList, s.logger, started)
}
