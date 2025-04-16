package admin

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createAdminSocketClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}

func TestAdminServer(t *testing.T) {
	t.Parallel()

	socketPath := t.TempDir() + "/admin_socket"
	cfg := &ServerConfig{Enabled: true, UnixSocketPath: socketPath}
	ctx := t.Context()
	go func() {
		err := StartAdminServer(ctx, cfg, logging.NewLogger("admin"))
		assert.NoError(t, err)
	}()

	client := createAdminSocketClient(socketPath)
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/ping", bytes.NewReader(nil))
		require.NoError(t, err)
		response, err := client.Do(req)
		response.Body.Close()
		return err == nil && response.StatusCode == http.StatusOK
	}, 5*time.Second, 200*time.Millisecond)

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	require.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())

	check := func(t *testing.T, lvl string, rspCode int, expectedLvl zerolog.Level) {
		t.Helper()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"http://unix/set_log_level?level="+lvl, bytes.NewReader(nil))
		require.NoError(t, err)
		response, err := client.Do(req)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, rspCode, response.StatusCode)
		require.Equal(t, expectedLvl, zerolog.GlobalLevel())
	}

	check(t, "warn", http.StatusOK, zerolog.WarnLevel)

	check(t, "invalid", http.StatusBadRequest, zerolog.WarnLevel)
}
