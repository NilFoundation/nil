package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

const (
	maxRequestContentLength = 1024 * 1024 * 32 // 32MB
	contentType             = "application/json"
	minSupportedRevision    = 900
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

func (hc *httpConn) RemoteAddr() string {
	return hc.url
}

func (hc *httpConn) Read() ([]*transport.Message, bool, error) {
	<-hc.closeCh
	return nil, false, io.EOF
}

func (hc *httpConn) Close() {
	hc.closeOnce.Do(func() { close(hc.closeCh) })
}

func (hc *httpConn) Closed() <-chan interface{} {
	return hc.closeCh
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

func newHTTPServerConn(r *http.Request, w http.ResponseWriter) transport.ServerCodec {
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
		check.PanicIfErr(json.NewEncoder(buf).Encode(transport.Message{
			ID:     json.RawMessage(id),
			Method: methodUp,
			Params: param,
		}))

		conn.Reader = buf
	} else {
		// It's a POST request or whatever, so process it like normal.
		conn.Reader = io.LimitReader(r.Body, maxRequestContentLength)
	}
	return transport.NewCodec(conn)
}

// Close does nothing and always returns nil.
func (t *httpServerConn) Close() error { return nil }

// RemoteAddr returns the peer address of the underlying connection.
func (t *httpServerConn) RemoteAddr() string {
	return t.r.RemoteAddr
}

// SetWriteDeadline does nothing and always returns nil.
func (t *httpServerConn) SetWriteDeadline(time.Time) error { return nil }
