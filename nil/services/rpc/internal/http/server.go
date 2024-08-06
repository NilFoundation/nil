package http

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

type Server struct {
	s *transport.Server
}

var _ http.Handler = (*Server)(nil)

func NewServer(s *transport.Server) *Server {
	return &Server{s: s}
}

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
	s.s.ServeSingleRequest(ctx, codec)
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

	// User-Agent is supported by server
	ua := r.Header.Get("User-Agent")
	if ua != "" {
		var uaPrefix string
		if strings.HasPrefix(ua, "nil-cli") {
			uaPrefix = "nil-cli/"
		} else if strings.HasPrefix(ua, "niljs") {
			uaPrefix = "niljs/"
		}

		version, hasVersion := strings.CutPrefix(ua, uaPrefix)
		if hasVersion {
			num, err := strconv.Atoi(version)
			if err == nil && num > 0 {
				if num < minSupportedRevision {
					err := fmt.Errorf("specified revision %d, minimum supported is %d", num, minSupportedRevision)
					return http.StatusUpgradeRequired, err
				}
			}
		}
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
