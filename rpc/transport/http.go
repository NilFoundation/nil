package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/NilFoundation/nil/common"
)

const (
	maxRequestContentLength = 1024 * 1024 * 32 // 32MB
	contentType             = "application/json"
)

type (
	remoteCtxKey    struct{}
	schemeCtxKey    struct{}
	localCtxKey     struct{}
	userAgentCtxKey struct{}
	originCtxKey    struct{}
)

// https://www.jsonrpc.org/historical/json-rpc-over-http.html#id13
var acceptedContentTypes = []string{contentType, "application/json-rpc", "application/jsonrequest"}

type httpConn struct {
	client    *http.Client
	url       string
	closeOnce sync.Once
	closeCh   chan interface{}
	mu        sync.Mutex // protects headers
	headers   http.Header
}

// httpConn is treated specially by Client.
func (hc *httpConn) WriteJSON(context.Context, interface{}) error {
	panic("writeJSON called on httpConn")
}

func (hc *httpConn) remoteAddr() string {
	return hc.url
}

func (hc *httpConn) Read() (*jsonrpcMessage, error) {
	<-hc.closeCh
	return nil, io.EOF
}

func (hc *httpConn) Close() {
	hc.closeOnce.Do(func() { close(hc.closeCh) })
}

func (hc *httpConn) closed() <-chan interface{} {
	return hc.closeCh
}

func (c *Client) sendHTTP(ctx context.Context, op *requestOp, msg interface{}) error {
	hc, ok := c.writeConn.(*httpConn)
	common.Require(ok)

	respBody, err := hc.doRequest(ctx, msg)
	if err != nil {
		return err
	}

	var respmsg jsonrpcMessage
	if err := json.Unmarshal(respBody, &respmsg); err != nil {
		return err
	}

	op.resp <- &respmsg
	return nil
}

func (hc *httpConn) doRequest(ctx context.Context, msg interface{}) ([]byte, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hc.url, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))

	// set headers
	hc.mu.Lock()
	req.Header = hc.headers.Clone()
	hc.mu.Unlock()

	// do request
	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// httpServerConn turns a HTTP connection into a Conn.
type httpServerConn struct {
	io.Reader
	io.Writer
	r *http.Request
}

func newHTTPServerConn(r *http.Request, w http.ResponseWriter) ServerCodec {
	conn := &httpServerConn{Writer: w, r: r}
	// if the request is a GET request, and the body is empty, we turn the request into fake json rpc request, see below
	// https://www.jsonrpc.org/historical/json-rpc-over-http.html#encoded-parameters
	// we however allow for non base64 encoded parameters to be passed
	if r.Method == http.MethodGet && r.ContentLength == 0 {
		// default id 1
		id := `1`
		idUp := r.URL.Query().Get("id")
		if idUp != "" {
			id = idUp
		}
		methodUp := r.URL.Query().Get("method")
		params, _ := url.QueryUnescape(r.URL.Query().Get("params"))
		param := []byte(params)
		if pb, err := base64.URLEncoding.DecodeString(params); err == nil {
			param = pb
		}

		buf := new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(jsonrpcMessage{
			ID:     json.RawMessage(id),
			Method: methodUp,
			Params: param,
		})
		common.FatalIf(err, nil, "Messages encode failed")

		conn.Reader = buf
	} else {
		// It's a POST request or whatever, so process it like normal.
		conn.Reader = io.LimitReader(r.Body, maxRequestContentLength)
	}
	return NewCodec(conn)
}

// Close does nothing and always returns nil.
func (t *httpServerConn) Close() error { return nil }

// RemoteAddr returns the peer address of the underlying connection.
func (t *httpServerConn) RemoteAddr() string {
	return t.r.RemoteAddr
}

// SetWriteDeadline does nothing and always returns nil.
func (t *httpServerConn) SetWriteDeadline(time.Time) error { return nil }

// ServeHTTP serves JSON-RPC requests over HTTP.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Permit dumb empty requests for remote health-checks (AWS)
	if r.Method == http.MethodGet && r.ContentLength == 0 && r.URL.RawQuery == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if code, err := validateRequest(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	// All checks passed, create a codec that reads directly from the request body
	// until EOF, writes the response to w, and orders the server to process a
	// single request.
	ctx := r.Context()
	ctx = context.WithValue(ctx, remoteCtxKey{}, r.RemoteAddr)
	ctx = context.WithValue(ctx, schemeCtxKey{}, r.Proto)
	ctx = context.WithValue(ctx, localCtxKey{}, r.Host)
	if ua := r.Header.Get("User-Agent"); ua != "" {
		ctx = context.WithValue(ctx, userAgentCtxKey{}, ua)
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		ctx = context.WithValue(ctx, originCtxKey{}, origin)
	}

	w.Header().Set("Content-Type", contentType)
	codec := newHTTPServerConn(r, w)
	defer codec.Close()
	s.serveSingleRequest(ctx, codec)
}

// validateRequest returns a non-zero response code and error message if the
// request is invalid.
func validateRequest(r *http.Request) (int, error) {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}
	if r.ContentLength > maxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, maxRequestContentLength)
		return http.StatusRequestEntityTooLarge, err
	}
	// Allow OPTIONS and GET (regardless of content-type)
	if r.Method == http.MethodOptions || r.Method == http.MethodGet {
		return 0, nil
	}
	// Check content-type
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil {
		for _, accepted := range acceptedContentTypes {
			if accepted == mt {
				return 0, nil
			}
		}
	}
	// Invalid content-type
	err := fmt.Errorf("invalid content type, only %s is supported", contentType)
	return http.StatusUnsupportedMediaType, err
}
