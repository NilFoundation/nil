package http

import (
	"io"
	"net/http"
	"time"
)

const (
	MaxRequestContentLength  = 1024 * 1024 * 32 // 32MB
	minSupportedRevision     = 771
	minSupportedNiljsVersion = "0.24.0"
	niljsClientVersionPrefix = "niljs/"
)

type (
	remoteCtxKey    struct{}
	schemeCtxKey    struct{}
	localCtxKey     struct{}
	userAgentCtxKey struct{}
	originCtxKey    struct{}
)

// HttpServerConn turns a HTTP connection into a Conn.
type HttpServerConn struct {
	io.Reader
	io.Writer
	Request *http.Request
}

// Close does nothing and always returns nil.
func (t *HttpServerConn) Close() error { return nil }

// RemoteAddr returns the peer address of the underlying connection.
func (t *HttpServerConn) RemoteAddr() string {
	return t.Request.RemoteAddr
}

// SetWriteDeadline does nothing and always returns nil.
func (t *HttpServerConn) SetWriteDeadline(time.Time) error { return nil }
