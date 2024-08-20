package http

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

type SingleRequestServer interface {
	ServeSingleRequest(ctx context.Context, r *http.Request, w http.ResponseWriter)
}

type Server struct {
	s                    SingleRequestServer
	contentType          string
	acceptedContentTypes []string
}

var _ http.Handler = (*Server)(nil)

func NewServer(s SingleRequestServer, contentType string, acceptedContentTypes []string) *Server {
	return &Server{
		s:                    s,
		contentType:          contentType,
		acceptedContentTypes: acceptedContentTypes,
	}
}

// ServeHTTP serves RPC requests over HTTP.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Permit dumb empty requests for remote health-checks (AWS)
	if r.Method == http.MethodGet && r.ContentLength == 0 && r.URL.RawQuery == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if code, err := s.validateRequest(r); err != nil {
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

	w.Header().Set("Content-Type", s.contentType)
	// TODO: pass HttpServerConn?
	s.s.ServeSingleRequest(ctx, r, w)
}

// validateRequest returns a non-zero response code and error message if the
// request is invalid.
func (s *Server) validateRequest(r *http.Request) (int, error) {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}
	if r.ContentLength > MaxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, MaxRequestContentLength)
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
				if num < minSupportedRevision && num != 1 {
					err := fmt.Errorf("specified revision %d, minimum supported is %d", num, minSupportedRevision)
					return http.StatusUpgradeRequired, err
				}
			}
		}
	}

	// Check content-type
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil {
		for _, accepted := range s.acceptedContentTypes {
			if accepted == mt {
				return 0, nil
			}
		}
	}
	// Invalid content-type
	err := fmt.Errorf("invalid content type, only %s is supported", s.contentType)
	return http.StatusUnsupportedMediaType, err
}
