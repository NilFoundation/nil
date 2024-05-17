package rpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

func StartRpcServer(ctx context.Context, cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) error {
	if cfg.Enabled {
		return startRegularRpcServer(ctx, cfg, rpcAPI, logger)
	}

	return nil
}

func startRegularRpcServer(ctx context.Context, cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) error {
	// register apis and create handler stack
	srv := transport.NewServer(cfg.TraceRequests, cfg.DebugSingleRequest, logger, cfg.RPCSlowLogThreshold)

	defer srv.Stop()

	var defaultAPIList []transport.API

	for _, api := range rpcAPI {
		if api.Namespace != "engine" {
			defaultAPIList = append(defaultAPIList, api)
		}
	}

	var apiFlags []string
	for _, flag := range cfg.API {
		if flag != "engine" {
			apiFlags = append(apiFlags, flag)
		}
	}

	if err := transport.RegisterApisFromWhitelist(defaultAPIList, apiFlags, srv, logger); err != nil {
		return fmt.Errorf("could not start register RPC apis: %w", err)
	}

	event := logger.Info()
	httpHandler := transport.NewHTTPHandlerStack(srv, cfg.HttpCORSDomain, cfg.HttpVirtualHost, cfg.HttpCompression)

	httpEndpoint := "tcp://" + net.JoinHostPort(cfg.HttpListenAddress, strconv.Itoa(cfg.HttpPort))
	if cfg.HttpURL != "" {
		httpEndpoint = cfg.HttpURL
	}
	listener, httpAddr, err := transport.StartHTTPEndpoint(httpEndpoint, &transport.HttpEndpointConfig{
		Timeouts: cfg.HTTPTimeouts,
	}, httpHandler)
	if err != nil {
		return fmt.Errorf("could not start RPC api: %w", err)
	}
	event = event.Str("http.url", httpAddr.String())
	defer func() { //nolint:contextcheck
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = listener.Shutdown(shutdownCtx)
		logger.Info().Str("url", httpAddr.String()).Msg("HTTP endpoint closed")
	}()

	event.Msg("JsonRpc endpoint opened")
	<-ctx.Done()
	logger.Info().Msg("Exiting...")
	return nil
}
