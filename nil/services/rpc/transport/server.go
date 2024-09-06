package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	nil_http "github.com/NilFoundation/nil/nil/services/rpc/internal/http"
	mapset "github.com/deckarep/golang-set"
	"github.com/rs/zerolog"
)

const (
	MetadataApi             = "rpc"
	defaultBatchConcurrency = 2
)

// Server is an RPC server.
type Server struct {
	services serviceRegistry
	run      int32
	codecs   mapset.Set // mapset.Set[ServerCodec] requires go 1.20

	batchConcurrency    uint
	traceRequests       bool // Whether to print requests at INFO level
	debugSingleRequest  bool // Whether to print requests at INFO level
	batchLimit          int  // Maximum number of requests in a batch
	logger              zerolog.Logger
	rpcSlowLogThreshold time.Duration
}

// NewServer creates a new server instance with no registered handlers.
func NewServer(traceRequests, debugSingleRequest bool, logger zerolog.Logger, rpcSlowLogThreshold time.Duration) *Server {
	server := &Server{
		services: serviceRegistry{logger: logger}, codecs: mapset.NewSet(), run: 1, batchConcurrency: defaultBatchConcurrency,
		traceRequests: traceRequests, debugSingleRequest: debugSingleRequest, logger: logger, rpcSlowLogThreshold: rpcSlowLogThreshold,
	}

	// Register the default service providing meta-information about the RPC service such
	// as the services and methods it offers.
	check.PanicIfErr(server.RegisterName(MetadataApi, &RPCService{server: server}))
	return server
}

// RegisterName creates a service for the given receiver type under the given name. When no
// methods on the given receiver match the criteria to be a RPC method an error is returned.
// Otherwise, a new service is created and added to the service collection this server provides to clients.
func (s *Server) RegisterName(name string, receiver interface{}) error {
	return s.services.registerName(name, receiver)
}

// SetBatchLimit sets limit of number of requests in a batch
func (s *Server) SetBatchLimit(limit int) {
	s.batchLimit = limit
}

func newHTTPServerConn(r *http.Request, w http.ResponseWriter) ServerCodec {
	conn := &nil_http.HttpServerConn{Writer: w, Request: r}
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
		check.PanicIfErr(json.NewEncoder(buf).Encode(Message{
			ID:     json.RawMessage(id),
			Method: methodUp,
			Params: param,
		}))

		conn.Reader = buf
	} else {
		// It's a POST request or whatever, so process it like normal.
		conn.Reader = io.LimitReader(r.Body, nil_http.MaxRequestContentLength)
	}
	return NewCodec(conn)
}

// ServeSingleRequest reads and processes a single RPC request from the given codec. This
// is used to serve HTTP connections.
func (s *Server) ServeSingleRequest(ctx context.Context, r *http.Request, w http.ResponseWriter) { // codec ServerCodec) {
	codec := newHTTPServerConn(r, w)
	defer codec.Close()

	// Don't serve if the server is stopped.
	if atomic.LoadInt32(&s.run) == 0 {
		return
	}

	h := newHandler(ctx, codec, &s.services, s.batchConcurrency, s.traceRequests, s.logger, s.rpcSlowLogThreshold)

	reqs, batch, err := codec.Read()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = codec.WriteJSON(ctx, errorMessage(&invalidMessageError{"parse error"}))
		}
		return
	}
	if batch {
		if s.batchLimit > 0 && len(reqs) > s.batchLimit {
			_ = codec.WriteJSON(ctx, errorMessage(fmt.Errorf("batch limit %d exceeded (can increase by --rpc.batch.limit). Requested batch of size: %d", s.batchLimit, len(reqs))))
		} else {
			h.handleBatch(reqs)
		}
	} else {
		h.handleMsg(reqs[0])
	}
}

// Stop stops reading new requests, waits for stopPendingRequestTimeout to allow pending
// requests to finish, then closes all codecs that will cancel pending requests.
func (s *Server) Stop() {
	if atomic.CompareAndSwapInt32(&s.run, 1, 0) {
		s.logger.Info().Msg("RPC server shutting down")
		s.codecs.Each(func(c interface{}) bool {
			if codec, ok := c.(ServerCodec); ok {
				codec.Close()
			}
			return true
		})
	}
}

// RPCService gives meta-information about the server.
// e.g., gives information about the loaded modules.
type RPCService struct {
	server *Server
}

// Modules returns the list of RPC services with their version number
func (s *RPCService) Modules() map[string]string {
	s.server.services.mu.Lock()
	defer s.server.services.mu.Unlock()

	modules := make(map[string]string)
	for name := range s.server.services.services {
		modules[name] = "1.0"
	}
	return modules
}
