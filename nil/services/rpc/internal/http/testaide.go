//go:build test

package http

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	applicationJsonContentType = "application/json"
	plantTextContentType       = "plain/text"
)

type StoppableSingleRequestServer interface {
	SingleRequestServer
	ServerStopper
}

func DoTestServer(t *testing.T, createServerAndLogger func(t *testing.T, conf *HttpConfig) (*StoppableSingleRequestServer, zerolog.Logger)) {
	t.Helper()

	launchServer := func(t *testing.T, conf *HttpConfig) *httpServer {
		t.Helper()

		server, logger := createServerAndLogger(t, conf)
		assert.NotEmpty(t, server)
		return createAndStartServer(t, *server, conf, logger)
	}

	// TestCorsHandler makes sure CORS are properly handled on the http server.
	t.Run("TestCorsHandler", func(t *testing.T) {
		srv := launchServer(t, &HttpConfig{CorsAllowedOrigins: []string{"test", "test.com"}})
		defer srv.stop()
		u := "http://" + srv.listenAddr()

		resp := rpcRequest(t, u, "origin", "test.com")
		defer resp.Body.Close()
		assert.Equal(t, "test.com", resp.Header.Get("Access-Control-Allow-Origin"))

		resp2 := rpcRequest(t, u, "origin", "bad")
		defer resp2.Body.Close()
		assert.Equal(t, "", resp2.Header.Get("Access-Control-Allow-Origin"))
	})

	// TestVhosts makes sure vhosts is properly handled on the http server.
	t.Run("TestVhosts", func(t *testing.T) {
		t.Parallel()

		srv := launchServer(t, &HttpConfig{Vhosts: []string{"test"}})
		defer srv.stop()
		u := "http://" + srv.listenAddr()

		resp := rpcRequest(t, u, "host", "test")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp2 := rpcRequest(t, u, "host", "bad")
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
	})

	// TestVhostsAny makes sure vhosts any is properly handled on the http server.
	t.Run("TestVhostsAny", func(t *testing.T) {
		t.Parallel()

		srv := launchServer(t, &HttpConfig{Vhosts: []string{"any"}})
		defer srv.stop()
		url := "http://" + srv.listenAddr()

		resp := rpcRequest(t, url, "host", "test")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp2 := rpcRequest(t, url, "host", "bad")
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func createAndStartServer(t *testing.T, server StoppableSingleRequestServer, conf *HttpConfig, logger zerolog.Logger) *httpServer {
	t.Helper()

	srv := newHTTPServer(logger, httpcfg.DefaultHTTPTimeouts)
	assert.NoError(t, srv.enableRPC(server, *conf))
	assert.NoError(t, srv.setListenAddr("localhost", 0))
	assert.NoError(t, srv.start())
	return srv
}

// rpcRequest performs a JSON-RPC request to the given URL.
func rpcRequest(t *testing.T, url string, extraHeaders ...string) *http.Response {
	t.Helper()

	// Create the request.
	body := bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"rpc_modules","params":[]}`))
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		t.Fatal("could not create http request:", err)
	}
	req.Header.Set("Content-Type", applicationJsonContentType)

	// Apply extra headers.
	require.Zero(t, len(extraHeaders)%2, "odd extraHeaders length")
	for i := 0; i < len(extraHeaders); i += 2 {
		key, value := extraHeaders[i], extraHeaders[i+1]
		if strings.EqualFold(key, "host") {
			req.Host = value
		} else {
			req.Header.Set(key, value)
		}
	}

	// Perform the request.
	t.Logf("checking RPC/HTTP on %s %v", url, extraHeaders)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// enableRPC turns on RPC over HTTP on the server.
func (h *httpServer) enableRPC(srv StoppableSingleRequestServer, config HttpConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rpcAllowed() {
		return errors.New("RPC over HTTP is already enabled")
	}

	h.httpConfig = config
	h.httpHandler.Store(&rpcHandler{
		Handler: NewHTTPHandlerStack(
			NewServer(srv, applicationJsonContentType, []string{applicationJsonContentType}),
			config.CorsAllowedOrigins,
			config.Vhosts,
			config.Compression),
		serverStopper: srv,
	})
	return nil
}
