package indexer

import (
	"context"
	"fmt"
	driver2 "github.com/NilFoundation/nil/nil/services/indexer/driver"
	types2 "github.com/NilFoundation/nil/nil/services/indexer/types"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/badger"
	"github.com/NilFoundation/nil/nil/services/indexer/clickhouse"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type Config struct {
	UseBadger    bool   `yaml:"use-badger,omitempty"`    //nolint:tagliatelle
	OwnEndpoint  string `yaml:"own-endpoint,omitempty"`  //nolint:tagliatelle
	NodeEndpoint string `yaml:"node-endpoint,omitempty"` //nolint:tagliatelle
	DbEndpoint   string `yaml:"db-endpoint,omitempty"`   //nolint:tagliatelle
	DbName       string `yaml:"db-name,omitempty"`       //nolint:tagliatelle
	DbUser       string `yaml:"db-user,omitempty"`       //nolint:tagliatelle
	DbPassword   string `yaml:"db-password,omitempty"`   //nolint:tagliatelle
	DbPath       string `yaml:"db-path,omitempty"`       //nolint:tagliatelle
}

const (
	OwnEndpointDefault  = "tcp://127.0.0.1:8528"
	NodeEndpointDefault = "http://127.0.0.1:8529"
	DbEndpointDefault   = "127.0.0.1:9000"
	DbNameDefault       = "nil_database"
	DbUserDefault       = "default"
	DbPasswordDefault   = ""
	DbPathDefault       = "indexer.db"
)

func (c *Config) ResetToDefault() {
	c.UseBadger = false
	c.OwnEndpoint = OwnEndpointDefault
	c.NodeEndpoint = NodeEndpointDefault
	c.DbEndpoint = DbEndpointDefault
	c.DbName = DbNameDefault
	c.DbUser = DbUserDefault
	c.DbPassword = DbPasswordDefault
	c.DbPath = DbPathDefault
}

type Service struct {
	driver driver2.IndexerDriver
	client client.Client
}

type IndexerJsonRpc interface {
	GetAddressActions(address types.Address, since db.Timestamp) ([]types2.AddressAction, error)
}

func NewService(ctx context.Context, cfg *Config, client client.Client) (*Service, error) {
	s := &Service{
		client: client,
	}

	var err error
	if cfg.UseBadger {
		s.driver, err = badger.NewBadgerDriver(cfg.DbPath)
	} else {
		s.driver, err = clickhouse.NewClickhouseDriver(ctx, cfg.DbEndpoint, cfg.DbUser, cfg.DbPassword, cfg.DbName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create storage driver: %w", err)
	}

	return s, nil
}

func (s *Service) GetAddressActions(address types.Address, since db.Timestamp) ([]types2.AddressAction, error) {
	return s.driver.FetchAddressActions(address, since)
}

func (s *Service) Run(ctx context.Context, cfg *Config) error {
	return s.startRpcServer(ctx, cfg.OwnEndpoint)
}

func (s *Service) GetRpcApi() transport.API {
	return transport.API{
		Namespace: "indexer",
		Public:    true,
		Service:   IndexerJsonRpc(s),
		Version:   "1.0",
	}
}

func (s *Service) startRpcServer(ctx context.Context, endpoint string) error {
	logger := logging.NewLogger("RPC")
	logger.Level(zerolog.InfoLevel)

	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
	}

	apiList := []transport.API{s.GetRpcApi()}
	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger, nil)
}
