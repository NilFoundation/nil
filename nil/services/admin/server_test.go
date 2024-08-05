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
	"github.com/stretchr/testify/require"
)

func TestAdminServer(t *testing.T) {
	t.Parallel()

	socketPath := t.TempDir() + "/admin_socket"
	cfg := &ServerConfig{Enabled: true, UnixSocketPath: socketPath}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = StartAdminServer(ctx, cfg, logging.NewLogger("admin"))
	}()

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	require.Eventually(t, func() bool {
		response, err := client.Post("http://unix/ping", "application/json", bytes.NewReader(nil))
		response.Body.Close()
		return err == nil && response.StatusCode == http.StatusOK
	}, 5*time.Second, 200*time.Millisecond)

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	require.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())

	check := func(t *testing.T, lvl string, rspCode int, expectedLvl zerolog.Level) {
		t.Helper()

		response, err := client.Post("http://unix/set_log_level?level="+lvl, "application/json", bytes.NewReader(nil))
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, rspCode, response.StatusCode)
		require.Equal(t, expectedLvl, zerolog.GlobalLevel())
	}

	check(t, "warn", http.StatusOK, zerolog.WarnLevel)

	check(t, "invalid", http.StatusBadRequest, zerolog.WarnLevel)
}
