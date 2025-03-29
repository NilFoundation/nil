package rpc

import (
	"context"
	"fmt"
	net_http "net/http"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/internal/http"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/rpc/transport/rpccfg"
)

func StartRpcServer(
	ctx context.Context,
	cfg *httpcfg.HttpCfg,
	rpcAPI []transport.API,
	logger logging.Logger,
	started chan<- struct{},
) error {
	// register apis and create handler stack
	srv := transport.NewServer(
		cfg.TraceRequests, cfg.DebugSingleRequest, logger, cfg.RPCSlowLogThreshold, cfg.KeepHeaders)

	defer srv.Stop()

	var defaultAPIList []transport.API

	for _, api := range rpcAPI {
		if api.Namespace != "engine" {
			defaultAPIList = append(defaultAPIList, api)
		}
	}

	if err := transport.RegisterApisFromWhitelist(defaultAPIList, nil, srv, logger); err != nil {
		return fmt.Errorf("could not start register RPC apis: %w", err)
	}

	httpEndpoint := cfg.HttpURL

	basicHttpSrv := http.NewServer(srv, rpccfg.ContentType, rpccfg.AcceptedContentTypes)
	var httpHandler net_http.Handler = basicHttpSrv
	if !strings.HasPrefix(httpEndpoint, "unix://") {
		httpHandler = http.NewHTTPHandlerStack(
			basicHttpSrv,
			cfg.HttpCORSDomain,
			nil,
			cfg.HttpCompression)
	}

	listener, httpAddr, err := http.StartHTTPEndpoint(httpEndpoint, &http.HttpEndpointConfig{
		Timeouts: cfg.HTTPTimeouts,
	}, httpHandler)
	if err != nil {
		return fmt.Errorf("could not start RPC api: %w", err)
	}

	defer func() { //nolint:contextcheck
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Info().Stringer(logging.FieldUrl, httpAddr).Msg("JsonRPC endpoint closing...")
		_ = listener.Shutdown(shutdownCtx)
		logger.Info().Stringer(logging.FieldUrl, httpAddr).Msg("JsonRPC endpoint closed.")
	}()

	logger.Info().Stringer(logging.FieldUrl, httpAddr).Msg("JsonRPC endpoint opened.")

	if started != nil {
		close(started)
	}

	<-ctx.Done()
	return nil
}
