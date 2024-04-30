package rpc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/transport"
)

func StartRpcServer(ctx context.Context, cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) error {
	if cfg.Enabled {
		return startRegularRpcServer(ctx, cfg, rpcAPI, logger)
	}

	return nil
}

func StartRpcServerWithJwtAuthentication(ctx context.Context, cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) error {
	if len(rpcAPI) == 0 {
		return nil
	}
	engineInfo, err := startAuthenticatedRpcServer(cfg, rpcAPI, logger)
	if err != nil {
		return err
	}
	go stopAuthenticatedRpcServer(ctx, engineInfo, logger)
	return nil
}

func startRegularRpcServer(ctx context.Context, cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) error {
	// register apis and create handler stack
	srv := transport.NewServer(cfg.RpcBatchConcurrency, cfg.TraceRequests, cfg.DebugSingleRequest, logger, cfg.RPCSlowLogThreshold)

	allowListForRPC, err := parseAllowListForRPC(cfg.RpcAllowListFilePath)
	if err != nil {
		return err
	}
	srv.SetAllowList(allowListForRPC)

	srv.SetBatchLimit(cfg.BatchLimit)

	defer srv.Stop()

	var defaultAPIList []transport.API

	for _, api := range rpcAPI {
		if api.Namespace != "engine" {
			defaultAPIList = append(defaultAPIList, api)
		}
	}

	var apiFlags []string
	for _, flag := range cfg.API {
		if flag != "engine" {
			apiFlags = append(apiFlags, flag)
		}
	}

	if err := transport.RegisterApisFromWhitelist(defaultAPIList, apiFlags, srv, logger); err != nil {
		return fmt.Errorf("could not start register RPC apis: %w", err)
	}

	event := logger.Info()
	httpHandler := transport.NewHTTPHandlerStack(srv, cfg.HttpCORSDomain, cfg.HttpVirtualHost, cfg.HttpCompression)
	apiHandler, err := createHandler(cfg, defaultAPIList, httpHandler, nil)
	if err != nil {
		return err
	}

	if cfg.HttpServerEnabled {
		httpEndpoint := fmt.Sprintf("tcp://%s:%d", cfg.HttpListenAddress, cfg.HttpPort)
		if cfg.HttpURL != "" {
			httpEndpoint = cfg.HttpURL
		}
		listener, httpAddr, err := transport.StartHTTPEndpoint(httpEndpoint, &transport.HttpEndpointConfig{
			Timeouts: cfg.HTTPTimeouts,
		}, apiHandler)
		if err != nil {
			return fmt.Errorf("could not start RPC api: %w", err)
		}
		event = event.Str("http.url", httpAddr.String())
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = listener.Shutdown(shutdownCtx)
			logger.Info().Str("url", httpAddr.String()).Msg("HTTP endpoint closed")
		}()
	}

	event.Msg("JsonRpc endpoint opened")
	<-ctx.Done()
	logger.Info().Msg("Exiting...")
	return nil
}

type engineInfo struct {
	Srv                *transport.Server
	EngineSrv          *transport.Server
	EngineListener     *http.Server
	EngineHttpEndpoint string
}

func startAuthenticatedRpcServer(cfg *httpcfg.HttpCfg, rpcAPI []transport.API, logger *zerolog.Logger) (*engineInfo, error) {
	srv := transport.NewServer(cfg.RpcBatchConcurrency, cfg.TraceRequests, cfg.DebugSingleRequest, logger, cfg.RPCSlowLogThreshold)

	engineListener, engineSrv, engineHttpEndpoint, err := createEngineListener(cfg, rpcAPI, logger)
	if err != nil {
		return nil, fmt.Errorf("could not start RPC api for engine: %w", err)
	}
	return &engineInfo{Srv: srv, EngineSrv: engineSrv, EngineListener: engineListener, EngineHttpEndpoint: engineHttpEndpoint}, nil
}

func stopAuthenticatedRpcServer(ctx context.Context, engineInfo *engineInfo, logger *zerolog.Logger) {
	defer func() {
		engineInfo.Srv.Stop()
		if engineInfo.EngineSrv != nil {
			engineInfo.EngineSrv.Stop()
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if engineInfo.EngineListener != nil {
			_ = engineInfo.EngineListener.Shutdown(shutdownCtx)
			logger.Info().Str("url", engineInfo.EngineHttpEndpoint).Msg("Engine HTTP endpoint close")
		}
	}()
	<-ctx.Done()
	logger.Info().Msg("Exiting Engine...")
}

// ObtainJWTSecret loads the jwt-secret, either from the provided config,
// or from the default location. If neither of those are present, it generates
// a new secret and stores to the default location.
func ObtainJWTSecret(cfg *httpcfg.HttpCfg, logger *zerolog.Logger) ([]byte, error) {
	// try reading from file
	logger.Info().Str("path", cfg.JWTSecretPath).Msg("Reading JWT secret")
	// If we run the rpcdaemon and datadir is not specified we just use jwt.hex in current directory.
	if len(cfg.JWTSecretPath) == 0 {
		cfg.JWTSecretPath = "jwt.hex"
	}
	if data, err := os.ReadFile(cfg.JWTSecretPath); err == nil {
		jwtSecret := common.FromHex(strings.TrimSpace(string(data)))
		if len(jwtSecret) == 32 {
			return jwtSecret, nil
		}
		logger.Error().Str("path", cfg.JWTSecretPath).Str("length", strconv.Itoa(len(jwtSecret))).Msg("Invalid JWT secret")
		return nil, errors.New("invalid JWT secret")
	}
	// Need to generate one
	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return nil, err
	}

	if err := os.WriteFile(cfg.JWTSecretPath, []byte(hexutil.Encode(jwtSecret)), 0600); err != nil {
		return nil, err
	}
	logger.Info().Str("path", cfg.JWTSecretPath).Msg("Generated JWT secret")
	return jwtSecret, nil
}

func createHandler(_ *httpcfg.HttpCfg, _ []transport.API, httpHandler http.Handler, jwtSecret []byte) (http.Handler, error) {
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if jwtSecret != nil && !transport.CheckJwtSecret(w, r, jwtSecret) {
			return
		}

		httpHandler.ServeHTTP(w, r)
	})

	return handler, nil
}

func createEngineListener(cfg *httpcfg.HttpCfg, engineApi []transport.API, logger *zerolog.Logger) (*http.Server, *transport.Server, string, error) {
	engineHttpEndpoint := fmt.Sprintf("tcp://%s:%d", cfg.AuthRpcHTTPListenAddress, cfg.AuthRpcPort)

	engineSrv := transport.NewServer(cfg.RpcBatchConcurrency, cfg.TraceRequests, cfg.DebugSingleRequest, logger, cfg.RPCSlowLogThreshold)

	if err := transport.RegisterApisFromWhitelist(engineApi, nil, engineSrv, logger); err != nil {
		return nil, nil, "", fmt.Errorf("could not start register RPC engine api: %w", err)
	}

	jwtSecret, err := ObtainJWTSecret(cfg, logger)
	if err != nil {
		return nil, nil, "", err
	}

	engineHttpHandler := transport.NewHTTPHandlerStack(engineSrv, nil /* authCors */, cfg.AuthRpcVirtualHost, cfg.HttpCompression)

	engineApiHandler, err := createHandler(cfg, engineApi, engineHttpHandler, jwtSecret)
	if err != nil {
		return nil, nil, "", err
	}

	engineListener, engineAddr, err := transport.StartHTTPEndpoint(engineHttpEndpoint, &transport.HttpEndpointConfig{
		Timeouts: cfg.AuthRpcTimeouts,
	}, engineApiHandler)
	if err != nil {
		return nil, nil, "", fmt.Errorf("could not start RPC api: %w", err)
	}

	logger.Info().Str("url", engineAddr.String()).Msg("HTTP endpoint opened for Engine API")

	return engineListener, engineSrv, engineAddr.String(), nil
}
