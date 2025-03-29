package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var ErrStopped = errors.New("stopped")

type HttpEndpointConfig struct {
	Timeouts httpcfg.HTTPTimeouts
}

// StartHTTPEndpoint starts the HTTP RPC endpoint.
func StartHTTPEndpoint(
	urlEndpoint string, cfg *HttpEndpointConfig, handler http.Handler,
) (*http.Server, net.Addr, error) {
	// start the HTTP listener
	var (
		listener net.Listener
		err      error
	)
	socketUrl, err := url.Parse(urlEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("malformatted http listen url %s: %w", urlEndpoint, err)
	}

	// We need to retry socket binding because tests may fail due to port already in use
	common.Retry(func() error {
		listener, err = net.Listen(socketUrl.Scheme, socketUrl.Host+socketUrl.EscapedPath())
		return err
	}, 3, 100*time.Millisecond, 400*time.Millisecond)
	if err != nil {
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
	addr := listener.Addr()

	// start the HTTP server
	go func() {
		serveErr := httpSrv.Serve(listener)
		if serveErr != nil && !isIgnoredHttpServerError(serveErr) {
			log.Warn().Err(serveErr).Stringer(logging.FieldUrl, addr).Msg("Failed to serve https endpoint")
		}
	}()

	return httpSrv, addr, err
}

func isIgnoredHttpServerError(serveErr error) bool {
	return errors.Is(serveErr, context.Canceled) ||
		errors.Is(serveErr, ErrStopped) ||
		errors.Is(serveErr, http.ErrServerClosed)
}

// CheckTimeouts ensures that timeout values are meaningful
func CheckTimeouts(timeouts *httpcfg.HTTPTimeouts) {
	if timeouts.ReadTimeout < time.Second {
		log.Warn().
			Str("provided", timeouts.WriteTimeout.String()).
			Str("updated", httpcfg.DefaultHTTPTimeouts.WriteTimeout.String()).
			Msg("Sanitizing invalid HTTP read timeout")
		timeouts.ReadTimeout = httpcfg.DefaultHTTPTimeouts.ReadTimeout
	}
	if timeouts.WriteTimeout < time.Second {
		log.Warn().
			Str("provided", timeouts.WriteTimeout.String()).
			Str("updated", httpcfg.DefaultHTTPTimeouts.WriteTimeout.String()).
			Msg("Sanitizing invalid HTTP write timeout")
		timeouts.WriteTimeout = httpcfg.DefaultHTTPTimeouts.WriteTimeout
	}
	if timeouts.IdleTimeout < time.Second {
		log.Warn().
			Str("provided", timeouts.IdleTimeout.String()).
			Str("updated", httpcfg.DefaultHTTPTimeouts.IdleTimeout.String()).
			Msg("Sanitizing invalid HTTP idle timeout")
		timeouts.IdleTimeout = httpcfg.DefaultHTTPTimeouts.IdleTimeout
	}
}
