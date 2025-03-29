package transport

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/internal/http"
)

func TestServer(t *testing.T) {
	t.Parallel()

	http.DoTestServer(
		t,
		func(t *testing.T, conf *http.HttpConfig) (*http.StoppableSingleRequestServer, logging.Logger) {
			t.Helper()

			logger := logging.NewLogger("Test server")
			// Create RPC server and handler.
			var apis []API
			server := NewServer(false /* traceRequests */, false /* traceSingleRequest */, logger, 0, []string{})
			if err := RegisterApisFromWhitelist(apis, conf.Modules, server, logger); err != nil {
				return nil, logger
			}
			var s http.StoppableSingleRequestServer = server
			return &s, logger
		})
}
