package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func confirmStatusCode(t *testing.T, got, want int) {
	t.Helper()
	statusToStr := func(code int) string {
		if name := http.StatusText(code); len(name) > 0 {
			return fmt.Sprintf("%d (%s)", code, name)
		}
		return strconv.Itoa(code)
	}
	require.Equalf(t, want, got, "response status code: got %s, want %s", statusToStr(got), statusToStr(want))
}

func confirmRequestValidationCode(t *testing.T, method, contentType, body string, expectedStatusCode int) {
	t.Helper()
	request := httptest.NewRequest(method, "http://url.com", strings.NewReader(body))
	if len(contentType) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	s := &Server{contentType: contentType, acceptedContentTypes: []string{contentType}}
	code, err := s.validateRequest(request)
	if code == 0 {
		require.NoError(t, err)
	} else {
		require.Errorf(t, err, "code %d", code)
	}
	confirmStatusCode(t, code, expectedStatusCode)
}

func TestHTTPErrorResponseWithDelete(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodDelete, applicationJsonContentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithPut(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPut, applicationJsonContentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithMaxContentLength(t *testing.T) {
	t.Parallel()

	body := make([]rune, MaxRequestContentLength+1)
	confirmRequestValidationCode(t,
		http.MethodPost, applicationJsonContentType, string(body), http.StatusRequestEntityTooLarge)
}

func TestHTTPErrorResponseWithEmptyContentType(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPost, "", "", http.StatusUnsupportedMediaType)
}

func TestHTTPErrorResponseWithValidRequest(t *testing.T) {
	t.Parallel()

	confirmRequestValidationCode(t, http.MethodPost, applicationJsonContentType, "", 0)
}

type stubServer struct {
	t *testing.T

	response  []byte
	wasCalled bool
}

func (s *stubServer) ServeSingleRequest(ctx context.Context, r *http.Request, w http.ResponseWriter) {
	l, err := w.Write(s.response)
	require.NoError(s.t, err)
	require.Len(s.t, s.response, l)

	s.wasCalled = true
}

func (s *stubServer) Stop() {}

func confirmHTTPRequestYieldsStatusCode(t *testing.T, method, contentType, body string, expectedStatusCode int) {
	t.Helper()
	s := &stubServer{t: t}
	ts := httptest.NewServer(NewServer(s, plantTextContentType, []string{plantTextContentType}))
	defer ts.Close()

	request, err := http.NewRequestWithContext(t.Context(), method, ts.URL+"?dummy", strings.NewReader(body))
	require.NoError(t, err, "failed to create a valid HTTP request")
	if len(contentType) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(request)
	require.NoError(t, err, "request failed")
	require.True(t, s.wasCalled)
	defer resp.Body.Close()
	confirmStatusCode(t, resp.StatusCode, expectedStatusCode)
}

func TestHTTPResponseWithEmptyGet(t *testing.T) {
	t.Parallel()

	confirmHTTPRequestYieldsStatusCode(t, http.MethodGet, "", "", http.StatusOK)
}

// This checks that MaxRequestContentLength is not applied to the response of a request.
func TestHTTPRespBodyUnlimited(t *testing.T) {
	t.Parallel()

	const respLength = MaxRequestContentLength * 3
	response := []byte(strings.Repeat("x", respLength))

	s := &stubServer{t: t, response: response}
	ts := httptest.NewServer(NewServer(s, plantTextContentType, []string{plantTextContentType}))
	defer ts.Close()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"?dummy", strings.NewReader(""))
	require.NoError(t, err, "failed to create a valid HTTP request")
	request.Header.Set("Content-Type", plantTextContentType)
	resp, err := http.DefaultClient.Do(request)
	require.NoError(t, err, "request failed")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, s.wasCalled)
	require.Len(t, body, respLength, "response has wrong length")
}
