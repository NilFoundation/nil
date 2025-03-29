package http

import (
	"compress/gzip"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/gorilla/handlers"
)

// HttpConfig is the HTTP configuration.
type HttpConfig struct {
	Modules            []string
	CorsAllowedOrigins []string
	Vhosts             []string
	Compression        bool
	prefix             string // path prefix on which to mount http handler
}

type ServerStopper interface {
	Stop()
}

type rpcHandler struct {
	http.Handler
	serverStopper ServerStopper
}

type httpServer struct {
	logger   logging.Logger
	timeouts httpcfg.HTTPTimeouts

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener // non-nil when server is running

	// HTTP RPC handler things.
	httpConfig  HttpConfig
	httpHandler atomic.Value // *rpcHandler

	// These are set by setListenAddr.
	endpoint string
	host     string
	port     int

	handlerNames map[string]string
}

func newHTTPServer(logger logging.Logger, timeouts httpcfg.HTTPTimeouts) *httpServer {
	h := &httpServer{logger: logger, timeouts: timeouts, handlerNames: make(map[string]string)}

	h.httpHandler.Store((*rpcHandler)(nil))
	return h
}

// setListenAddr configures the listening address of the server.
// The address can only be set while the server isn't running.
func (h *httpServer) setListenAddr(host string, port int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil && (host != h.host || port != h.port) {
		return fmt.Errorf("HTTP server already running on %s", h.endpoint)
	}

	h.host, h.port = host, port
	h.endpoint = fmt.Sprintf("%s:%d", host, port)
	return nil
}

// listenAddr returns the listening address of the server.
func (h *httpServer) listenAddr() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.listener != nil {
		return h.listener.Addr().String()
	}
	return h.endpoint
}

func (h *httpServer) listen() (net.Listener, error) {
	if strings.HasPrefix(h.endpoint, "unix://") {
		socketPath := strings.TrimPrefix(h.endpoint, "unix://")

		socketDir := filepath.Dir(socketPath)
		if err := os.MkdirAll(socketDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create directory for Unix socket: %w", err)
		}

		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove existing Unix socket: %w", err)
		}
		return net.Listen("unix", socketPath)
	} else {
		return net.Listen("tcp", h.endpoint)
	}
}

// start starts the HTTP server if it is enabled and not already running.
func (h *httpServer) start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.endpoint == "" || h.listener != nil {
		return nil // already running or not configured
	}

	// Initialize the server.
	h.server = &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	if h.timeouts != (httpcfg.HTTPTimeouts{}) {
		CheckTimeouts(&h.timeouts)
		h.server.ReadTimeout = h.timeouts.ReadTimeout
		h.server.WriteTimeout = h.timeouts.WriteTimeout
		h.server.IdleTimeout = h.timeouts.IdleTimeout
	}

	// Start the server.
	listener, err := h.listen()
	if err != nil {
		// If the server fails to start, we need to clear out the RPC
		// configuration, so they can be configured another time.
		h.disableRPC()
		return err
	}
	h.listener = listener
	go func() {
		_ = h.server.Serve(listener)
	}()

	// if server is websocket only, return after logging
	if !h.rpcAllowed() {
		return nil
	}
	// Log http endpoint.
	h.logger.Info().
		Str("endpoint", listener.Addr().String()).
		Str("prefix", h.httpConfig.prefix).
		Str("cors", strings.Join(h.httpConfig.CorsAllowedOrigins, ",")).
		Str("vhosts", strings.Join(h.httpConfig.Vhosts, ",")).
		Msg("HTTP server started")

	// Log all handlers mounted on server.
	paths := make([]string, len(h.handlerNames))
	i := 0
	for path := range h.handlerNames {
		paths[i] = path
		i++
	}
	sort.Strings(paths)
	logged := make(map[string]bool, len(paths))
	for _, path := range paths {
		name := h.handlerNames[path]
		if !logged[name] {
			h.logger.Info().Str("url", "http://"+listener.Addr().String()+path).Msg(name + " enabled")
			logged[name] = true
		}
	}
	return nil
}

func (h *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if http-rpc is enabled, try to serve request
	rpc, ok := h.httpHandler.Load().(*rpcHandler)
	if ok && rpc != nil {
		if checkPath(r, h.httpConfig.prefix) {
			rpc.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

// checkPath checks whether a given request URL matches a given path prefix.
func checkPath(r *http.Request, path string) bool {
	// if no prefix has been specified, request URL must be on root
	if path == "" {
		return r.URL.Path == "/"
	}
	// otherwise, check to make sure prefix matches
	return len(r.URL.Path) >= len(path) && r.URL.Path[:len(path)] == path
}

// stop shuts down the HTTP server.
func (h *httpServer) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.doStop()
}

func (h *httpServer) doStop() {
	if h.listener == nil {
		return // not running
	}

	// Shut down the server.
	httpHandler, ok := h.httpHandler.Load().(*rpcHandler)
	if ok && httpHandler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		httpHandler.serverStopper.Stop()
	}
	_ = h.server.Shutdown(context.Background())
	_ = h.listener.Close()
	h.logger.Info().Str("endpoint", h.listener.Addr().String()).Msg("HTTP server stopped")

	// Clear out everything to allow re-configuring it later.
	h.host, h.port, h.endpoint = "", 0, ""
	h.server, h.listener = nil, nil
}

// disableRPC stops the HTTP RPC handler. This is internal, the caller must hold h.mu.
func (h *httpServer) disableRPC() bool {
	handler, ok := h.httpHandler.Load().(*rpcHandler)
	if ok && handler != nil {
		h.httpHandler.Store((*rpcHandler)(nil))
		handler.serverStopper.Stop()
	}
	return handler != nil
}

// rpcAllowed returns true when RPC over HTTP is enabled.
func (h *httpServer) rpcAllowed() bool {
	handler, ok := h.httpHandler.Load().(*rpcHandler)
	return ok && handler != nil
}

func newCorsHandler(srv http.Handler, allowedOrigins []string) http.Handler {
	// disable CORS support if a user has not specified a custom CORS configuration
	if len(allowedOrigins) == 0 {
		return srv
	}

	return handlers.CORS(
		handlers.AllowedOrigins(allowedOrigins),
		handlers.AllowedHeaders([]string{nilJsVersionHeader, "Content-Type"}), // this headers uses nil.js
		handlers.AllowedMethods([]string{http.MethodPost, http.MethodGet}),
		handlers.MaxAge(600),
	)(srv)
}

// virtualHostHandler is a handler which validates the Host-header of incoming requests.
// Using virtual hosts can help prevent DNS rebinding attacks, where a 'random' domain name points to
// the service ip address (but without CORS headers). By verifying the targeted virtual host, we can
// ensure that it's a destination that the node operator has defined.
type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}

// NewHTTPHandlerStack returns wrapped http-related handlers
func NewHTTPHandlerStack(srv http.Handler, cors []string, vhosts []string, compression bool) http.Handler {
	// Wrap the CORS-handler within a host-handler
	handler := newCorsHandler(srv, cors)
	handler = newVHostHandler(vhosts, handler)
	if compression {
		handler = newGzipHandler(handler)
	}
	return handler
}

// ServeHTTP serves RPC requests over HTTP, implements http.Handler
func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if r.Host is not set, we can continue serving since a browser would set the Host header
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// Either invalid (too many colons) or no port specified
		host = r.Host
	}
	if ipAddr := net.ParseIP(host); ipAddr != nil {
		// It's an IP address, we can serve that
		h.next.ServeHTTP(w, r)
		return
	}

	// Not an IP address, but a hostname. Need to validate
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts["any"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts[strings.ToLower(host)]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "invalid host specified", http.StatusForbidden)
}

func newGzipHandler(next http.Handler) http.Handler {
	return handlers.CompressHandlerLevel(next, gzip.DefaultCompression)
}
