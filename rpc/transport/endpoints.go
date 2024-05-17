package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var ErrStopped = errors.New("stopped")

type HttpEndpointConfig struct {
	Timeouts rpccfg.HTTPTimeouts
}

// StartHTTPEndpoint starts the HTTP RPC endpoint.
func StartHTTPEndpoint(urlEndpoint string, cfg *HttpEndpointConfig, handler http.Handler) (*http.Server, net.Addr, error) {
	// start the HTTP listener
	var (
		listener net.Listener
		err      error
	)
	socketUrl, err := url.Parse(urlEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("malformatted http listen url %s: %w", urlEndpoint, err)
	}
	if listener, err = net.Listen(socketUrl.Scheme, socketUrl.Host+socketUrl.EscapedPath()); err != nil {
		return nil, nil, err
	}
	// make sure timeout values are meaningful
	CheckTimeouts(&cfg.Timeouts)
	// create the http2 server for handling h2c
	h2 := &http2.Server{}
	// enable h2c support
	handler = h2c.NewHandler(handler, h2)
	// Bundle the http server
	httpSrv := &http.Server{
		Handler:           handler,
		ReadTimeout:       cfg.Timeouts.ReadTimeout,
		WriteTimeout:      cfg.Timeouts.WriteTimeout,
		IdleTimeout:       cfg.Timeouts.IdleTimeout,
		ReadHeaderTimeout: cfg.Timeouts.ReadTimeout,
	}
	// start the HTTP server
	go func() {
		serveErr := httpSrv.Serve(listener)
		if serveErr != nil && !isIgnoredHttpServerError(serveErr) {
			log.Warn().Str("err", serveErr.Error()).Msg("Failed to serve https endpoint")
		}
	}()
	return httpSrv, listener.Addr(), err
}

func isIgnoredHttpServerError(serveErr error) bool {
	return errors.Is(serveErr, context.Canceled) || errors.Is(serveErr, ErrStopped) || errors.Is(serveErr, http.ErrServerClosed)
}

// checkModuleAvailability checks that all names given in modules are actually
// available API services. It assumes that the MetadataApi module ("rpc") is always available;
// the registration of this "rpc" module happens in NewServer() and is thus common to all endpoints.
func checkModuleAvailability(modules []string, apis []API) (bad, available []string) {
	availableSet := make(map[string]struct{})
	for _, api := range apis {
		if _, ok := availableSet[api.Namespace]; !ok {
			availableSet[api.Namespace] = struct{}{}
			available = append(available, api.Namespace)
		}
	}
	for _, name := range modules {
		if _, ok := availableSet[name]; !ok && name != MetadataApi {
			bad = append(bad, name)
		}
	}
	return bad, available
}

// CheckTimeouts ensures that timeout values are meaningful
func CheckTimeouts(timeouts *rpccfg.HTTPTimeouts) {
	if timeouts.ReadTimeout < time.Second {
		log.Warn().Str("provided", timeouts.WriteTimeout.String()).Str("updated", rpccfg.DefaultHTTPTimeouts.WriteTimeout.String()).Msg("Sanitizing invalid HTTP read timeout")
		timeouts.ReadTimeout = rpccfg.DefaultHTTPTimeouts.ReadTimeout
	}
	if timeouts.WriteTimeout < time.Second {
		log.Warn().Str("provided", timeouts.WriteTimeout.String()).Str("updated", rpccfg.DefaultHTTPTimeouts.WriteTimeout.String()).Msg("Sanitizing invalid HTTP write timeout")
		timeouts.WriteTimeout = rpccfg.DefaultHTTPTimeouts.WriteTimeout
	}
	if timeouts.IdleTimeout < time.Second {
		log.Warn().Str("provided", timeouts.IdleTimeout.String()).Str("updated", rpccfg.DefaultHTTPTimeouts.IdleTimeout.String()).Msg("Sanitizing invalid HTTP idle timeout")
		timeouts.IdleTimeout = rpccfg.DefaultHTTPTimeouts.IdleTimeout
	}
}
