package transport

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/common/check"
	mapset "github.com/deckarep/golang-set"
	"github.com/rs/zerolog"
)

const MetadataApi = "rpc"

// Server is an RPC server.
type Server struct {
	services serviceRegistry
	run      int32
	codecs   mapset.Set // mapset.Set[ServerCodec] requires go 1.20

	traceRequests       bool // Whether to print requests at INFO level
	debugSingleRequest  bool // Whether to print requests at INFO level
	logger              zerolog.Logger
	rpcSlowLogThreshold time.Duration
}

// NewServer creates a new server instance with no registered handlers.
func NewServer(traceRequests, debugSingleRequest bool, logger zerolog.Logger, rpcSlowLogThreshold time.Duration) *Server {
	server := &Server{
		services: serviceRegistry{logger: logger}, codecs: mapset.NewSet(), run: 1,
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

// ServeCodec reads incoming requests from codec, calls the appropriate callback and writes
// the response back using the given codec. It will block until the codec is closed or the
// server is stopped. In either case, the codec is closed.
//
// Note that codec options are no longer supported.
func (s *Server) ServeCodec(codec ServerCodec) {
	defer codec.Close()

	// Don't serve if the server is stopped.
	if atomic.LoadInt32(&s.run) == 0 {
		return
	}

	// Add the codec to the set, so it can be closed by Stop.
	s.codecs.Add(codec)
	defer s.codecs.Remove(codec)

	c := initClient(codec, s.logger)
	<-codec.closed()
	c.Close()
}

// serveSingleRequest reads and processes a single RPC request from the given codec. This
// is used to serve HTTP connections.
func (s *Server) serveSingleRequest(ctx context.Context, codec ServerCodec) {
	// Don't serve if the server is stopped.
	if atomic.LoadInt32(&s.run) == 0 {
		return
	}

	h := newHandler(ctx, codec, &s.services, s.traceRequests, s.logger, s.rpcSlowLogThreshold)
	defer h.close(io.EOF, nil)

	req, err := codec.Read()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = codec.WriteJSON(ctx, errorMessage(&invalidMessageError{"parse error"}))
		}
		return
	}
	h.handleMsg(req)
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
