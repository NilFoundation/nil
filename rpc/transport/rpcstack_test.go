package transport

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"

	"github.com/stretchr/testify/assert"
)

// TestCorsHandler makes sure CORS are properly handled on the http server.
func TestCorsHandler(t *testing.T) {
	srv := createAndStartServer(t, &httpConfig{CorsAllowedOrigins: []string{"test", "test.com"}})
	defer srv.stop()
	url := "http://" + srv.listenAddr()

	resp := rpcRequest(t, url, "origin", "test.com")
	defer resp.Body.Close()
	assert.Equal(t, "test.com", resp.Header.Get("Access-Control-Allow-Origin"))

	resp2 := rpcRequest(t, url, "origin", "bad")
	defer resp2.Body.Close()
	assert.Equal(t, "", resp2.Header.Get("Access-Control-Allow-Origin"))
}

// TestVhosts makes sure vhosts is properly handled on the http server.
func TestVhosts(t *testing.T) {
	srv := createAndStartServer(t, &httpConfig{Vhosts: []string{"test"}})
	defer srv.stop()
	url := "http://" + srv.listenAddr()

	resp := rpcRequest(t, url, "host", "test")
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	resp2 := rpcRequest(t, url, "host", "bad")
	defer resp2.Body.Close()
	assert.Equal(t, resp2.StatusCode, http.StatusForbidden)
}

// TestVhostsAny makes sure vhosts any is properly handled on the http server.
func TestVhostsAny(t *testing.T) {
	srv := createAndStartServer(t, &httpConfig{Vhosts: []string{"any"}})
	defer srv.stop()
	url := "http://" + srv.listenAddr()

	resp := rpcRequest(t, url, "host", "test")
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	resp2 := rpcRequest(t, url, "host", "bad")
	defer resp2.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func Test_checkPath(t *testing.T) {
	tests := []struct {
		req      *http.Request
		prefix   string
		expected bool
	}{
		{
			req:      &http.Request{URL: &url.URL{Path: "/test"}},
			prefix:   "/test",
			expected: true,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/testing"}},
			prefix:   "/test",
			expected: true,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/"}},
			prefix:   "/test",
			expected: false,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/fail"}},
			prefix:   "/test",
			expected: false,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/"}},
			prefix:   "",
			expected: true,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/fail"}},
			prefix:   "",
			expected: false,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/"}},
			prefix:   "/",
			expected: true,
		},
		{
			req:      &http.Request{URL: &url.URL{Path: "/testing"}},
			prefix:   "/",
			expected: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			assert.Equal(t, tt.expected, checkPath(tt.req, tt.prefix)) //nolint:scopelint
		})
	}
}

func createAndStartServer(t *testing.T, conf *httpConfig) *httpServer {
	t.Helper()

	logger := common.NewLogger("Test server", false)
	srv := newHTTPServer(logger, rpccfg.DefaultHTTPTimeouts)
	assert.NoError(t, srv.enableRPC(nil, *conf))
	assert.NoError(t, srv.setListenAddr("localhost", 0))
	assert.NoError(t, srv.start())
	return srv
}

// rpcRequest performs a JSON-RPC request to the given URL.
func rpcRequest(t *testing.T, url string, extraHeaders ...string) *http.Response {
	t.Helper()

	// Create the request.
	body := bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"rpc_modules","params":[]}`))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		t.Fatal("could not create http request:", err)
	}
	req.Header.Set("content-type", "application/json")

	// Apply extra headers.
	if len(extraHeaders)%2 != 0 {
		panic("odd extraHeaders length")
	}
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

// enableRPC turns on JSON-RPC over HTTP on the server.
func (h *httpServer) enableRPC(apis []API, config httpConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rpcAllowed() {
		return fmt.Errorf("JSON-RPC over HTTP is already enabled")
	}

	// Create RPC server and handler.
	srv := NewServer(50, false /* traceRequests */, false /* traceSingleRequest */, h.logger, 0)
	if err := RegisterApisFromWhitelist(apis, config.Modules, srv, false, h.logger); err != nil {
		return err
	}
	h.httpConfig = config
	h.httpHandler.Store(&rpcHandler{
		Handler: NewHTTPHandlerStack(srv, config.CorsAllowedOrigins, config.Vhosts, config.Compression),
		server:  srv,
	})
	return nil
}
