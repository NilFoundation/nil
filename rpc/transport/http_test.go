package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/rs/zerolog"
)

// largeRespService generates arbitrary-size JSON responses.
type largeRespService struct {
	length int
}

func (x largeRespService) LargeResp() string {
	return strings.Repeat("x", x.length)
}

// dialHTTPWithClient creates a new RPC client that connects to an RPC server over HTTP
// using the provided HTTP Client.
func dialHTTPWithClient(endpoint string, client *http.Client, logger *zerolog.Logger) (*Client, error) {
	// Sanity check URL, so we don't end up with a client that will fail every request.
	_, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	initctx := context.Background()
	headers := make(http.Header, 2)
	headers.Set("Accept", contentType)
	headers.Set("Content-Type", contentType)
	return newClient(initctx, func(context.Context) (ServerCodec, error) {
		hc := &httpConn{
			client:  client,
			headers: headers,
			url:     endpoint,
			closeCh: make(chan interface{}),
		}
		return hc, nil
	}, logger)
}

// dialHTTP creates a new RPC client that connects to an RPC server over HTTP.
func dialHTTP(endpoint string, logger *zerolog.Logger) (*Client, error) {
	return dialHTTPWithClient(endpoint, new(http.Client), logger)
}

func confirmStatusCode(t *testing.T, got, want int) {
	t.Helper()
	if got == want {
		return
	}
	if gotName := http.StatusText(got); len(gotName) > 0 {
		if wantName := http.StatusText(want); len(wantName) > 0 {
			t.Fatalf("response status code: got %d (%s), want %d (%s)", got, gotName, want, wantName)
		}
	}
	t.Fatalf("response status code: got %d, want %d", got, want)
}

func confirmRequestValidationCode(t *testing.T, method, contentType, body string, expectedStatusCode int) {
	t.Helper()
	request := httptest.NewRequest(method, "http://url.com", strings.NewReader(body))
	if len(contentType) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	code, err := validateRequest(request)
	if code == 0 {
		if err != nil {
			t.Errorf("validation: got error %v, expected nil", err)
		}
	} else if err == nil {
		t.Errorf("validation: code %d: got nil, expected error", code)
	}
	confirmStatusCode(t, code, expectedStatusCode)
}

func TestHTTPErrorResponseWithDelete(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodDelete, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithPut(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPut, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithMaxContentLength(t *testing.T) {
	t.Parallel()

	body := make([]rune, maxRequestContentLength+1)
	confirmRequestValidationCode(t,
		http.MethodPost, contentType, string(body), http.StatusRequestEntityTooLarge)
}

func TestHTTPErrorResponseWithEmptyContentType(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPost, "", "", http.StatusUnsupportedMediaType)
}

func TestHTTPErrorResponseWithValidRequest(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPost, contentType, "", 0)
}

func confirmHTTPRequestYieldsStatusCode(t *testing.T, method, contentType, body string, expectedStatusCode int) {
	t.Helper()
	s := Server{}
	ts := httptest.NewServer(&s)
	defer ts.Close()

	request, err := http.NewRequest(method, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create a valid HTTP request: %v", err)
	}
	if len(contentType) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	confirmStatusCode(t, resp.StatusCode, expectedStatusCode)
}

func TestHTTPResponseWithEmptyGet(t *testing.T) {
	t.Parallel()

	confirmHTTPRequestYieldsStatusCode(t, http.MethodGet, "", "", http.StatusOK)
}

// This checks that maxRequestContentLength is not applied to the response of a request.
func TestHTTPRespBodyUnlimited(t *testing.T) {
	t.Parallel()

	logger := common.NewLogger("Test server", false)
	const respLength = maxRequestContentLength * 3

	s := NewServer(false /* traceRequests */, false /* debugSingleRequests */, logger, 100)
	defer s.Stop()
	if err := s.RegisterName("test", largeRespService{respLength}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	defer ts.Close()

	c, err := dialHTTP(ts.URL, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var r string
	if err := c.Call(&r, "test_largeResp"); err != nil {
		t.Fatal(err)
	}
	if len(r) != respLength {
		t.Fatalf("response has wrong length %d, want %d", len(r), respLength)
	}
}
