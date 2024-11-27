package rpc

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/require"
)

func TestParseSocketUrl(t *testing.T) {
	t.Parallel()

	t.Run("sock", func(t *testing.T) {
		t.Parallel()

		socketUrl, err := url.Parse("unix:///some/file/path.sock")
		require.NoError(t, err)
		require.EqualValues(t, "/some/file/path.sock", socketUrl.Host+socketUrl.EscapedPath())
	})
	t.Run("tcp", func(t *testing.T) {
		t.Parallel()

		socketUrl, err := url.Parse("tcp://localhost:1234")
		require.NoError(t, err)
		require.EqualValues(t, "localhost:1234", socketUrl.Host+socketUrl.EscapedPath())
	})
}

type testApi struct {
	contextCancelled bool
}

func (s *testApi) Method(ctx context.Context) error {
	if ctx.Err() != nil {
		s.contextCancelled = true
	}
	return nil
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api := &testApi{contextCancelled: false}

	socketPath := GetSockPath(t)
	go func() {
		httpConfig := &httpcfg.HttpCfg{
			HttpURL:         socketPath,
			HttpCompression: true,
			TraceRequests:   true,
			HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		}
		_ = StartRpcServer(
			ctx,
			httpConfig,
			[]transport.API{{
				Namespace: "test",
				Version:   "1.0",
				Service:   api,
				Public:    true,
			}},
			logging.NewLogger("Test server"))
	}()

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", strings.TrimPrefix(socketPath, "unix://"))
			},
		},
	}

	req, err := http.NewRequest(
		http.MethodGet,
		"http://unix",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"test_method","params":[]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	require.Eventually(t, func() bool {
		resp, err := client.Do(req)
		if err != nil {
			return false
		}

		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		return true
	}, 2*time.Second, 100*time.Millisecond)

	require.False(t, api.contextCancelled)
}
