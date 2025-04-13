package http

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	t.Parallel()

	DoTestServer(
		t,
		func(t *testing.T, conf *HttpConfig) (*StoppableSingleRequestServer, logging.Logger) {
			t.Helper()

			logger := logging.NewLogger("Test server")
			var server StoppableSingleRequestServer = &stubServer{t: t}
			return &server, logger
		})
}

func Test_checkPath(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			assert.Equal(t, tt.expected, checkPath(tt.req, tt.prefix))
		})
	}
}
