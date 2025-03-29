package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
)

var defaultTimeout = 30 * time.Second

type adminServer struct {
	mux    *http.ServeMux
	cfg    *ServerConfig
	logger logging.Logger
}

func StartAdminServer(ctx context.Context, cfg *ServerConfig, logger logging.Logger) error {
	if !cfg.Enabled {
		return nil
	}

	srv := &adminServer{
		mux:    http.NewServeMux(),
		cfg:    cfg,
		logger: logger,
	}

	// GET http:/./set_log_level?level=info
	srv.mux.HandleFunc("/set_log_level", srv.setLogLevel)
	srv.mux.HandleFunc("/set_libp2p_log_level", srv.setLibp2pLogLevel)
	srv.mux.HandleFunc("/set_block_profile_rate", srv.setBlockProfileRate)
	srv.mux.HandleFunc("/set_cpu_profile_rate", srv.setCPUProfileRate)
	srv.mux.HandleFunc("/set_mutex_profile_fraction", srv.setMutexProfileFraction)
	srv.mux.HandleFunc("/set_mem_profile_rate", srv.setMemProfileRate)
	srv.mux.HandleFunc("/ping", srv.ping)

	if err := srv.serve(ctx); err != nil {
		return fmt.Errorf("error starting admin server: %w", err)
	}

	return nil
}

func (s *adminServer) serve(ctx context.Context) error {
	srv := http.Server{Handler: s.mux, ReadHeaderTimeout: defaultTimeout}

	// Drop dangling unix socket if server previously was not stopped correctly.
	if _, err := os.Stat(s.cfg.UnixSocketPath); err == nil {
		conn, err := net.Dial("unix", s.cfg.UnixSocketPath)
		if err != nil {
			var netErr *net.OpError
			if errors.As(err, &netErr) && errors.Is(netErr, syscall.ECONNREFUSED) {
				s.logger.Info().Msgf("Remove unused socket file: %s", s.cfg.UnixSocketPath)
				os.Remove(s.cfg.UnixSocketPath)
			} else {
				return fmt.Errorf("error connecting to socket: %w", err)
			}
		} else {
			// Close if socket is already open.
			// Error "already in use" will be returned by net.Listen.
			conn.Close()
		}
	}

	listener, err := net.Listen("unix", s.cfg.UnixSocketPath)
	if err != nil {
		return err
	}

	defer func() { //nolint:contextcheck
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.logger.Info().Msg("Stopping admin server...")
		_ = srv.Shutdown(shutdownCtx)
		s.logger.Info().Msg("Stopped admin server")
	}()

	go func() {
		s.logger.Info().Msgf("Serving admin handles at `%s`", s.cfg.UnixSocketPath)
		_ = srv.Serve(listener)
	}()

	<-ctx.Done()
	return nil
}

func (s *adminServer) writeResponse(w http.ResponseWriter, err error, okMsg string) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, okMsg)
	}
}

func (s *adminServer) setLogLevel(w http.ResponseWriter, r *http.Request) {
	lvl := r.URL.Query().Get("level")
	err := logging.TrySetupGlobalLevel(lvl)
	s.writeResponse(w, err, "set to "+lvl)
}

func (s *adminServer) setLibp2pLogLevel(w http.ResponseWriter, r *http.Request) {
	lvl := r.URL.Query().Get("level")
	err := logging.SetLibp2pLogLevel(lvl)
	s.writeResponse(w, err, "set to "+lvl)
}

func (s *adminServer) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *adminServer) setBlockProfileRate(w http.ResponseWriter, r *http.Request) {
	rateStr := r.URL.Query().Get("rate")
	rate, err := strconv.Atoi(rateStr)
	if err != nil {
		http.Error(w, "Invalid rate value", http.StatusBadRequest)
		return
	}
	runtime.SetBlockProfileRate(rate)
	s.writeResponse(w, nil, "SetBlockProfileRate set to "+rateStr)
}

func (s *adminServer) setCPUProfileRate(w http.ResponseWriter, r *http.Request) {
	rateStr := r.URL.Query().Get("rate")
	rate, err := strconv.Atoi(rateStr)
	if err != nil {
		http.Error(w, "Invalid rate value", http.StatusBadRequest)
		return
	}
	runtime.SetCPUProfileRate(rate)
	s.writeResponse(w, nil, "SetCPUProfileRate set to "+rateStr)
}

func (s *adminServer) setMutexProfileFraction(w http.ResponseWriter, r *http.Request) {
	fractionStr := r.URL.Query().Get("fraction")
	fraction, err := strconv.Atoi(fractionStr)
	if err != nil {
		http.Error(w, "Invalid fraction value", http.StatusBadRequest)
		return
	}
	runtime.SetMutexProfileFraction(fraction)
	s.writeResponse(w, nil, "SetMutexProfileFraction set to "+fractionStr)
}

func (s *adminServer) setMemProfileRate(w http.ResponseWriter, r *http.Request) {
	rateStr := r.URL.Query().Get("rate")
	rate, err := strconv.Atoi(rateStr)
	if err != nil {
		http.Error(w, "Invalid rate value", http.StatusBadRequest)
		return
	}
	runtime.MemProfileRate = rate
	s.writeResponse(w, nil, "MemProfileRate set to "+rateStr)
}
