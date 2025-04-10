package indexer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/badger"
	"github.com/NilFoundation/nil/nil/services/indexer/clickhouse"
	indexerdriver "github.com/NilFoundation/nil/nil/services/indexer/driver"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Config struct {
	UseBadger   bool   `yaml:"use-badger,omitempty"`   //nolint:tagliatelle
	OwnEndpoint string `yaml:"own-endpoint,omitempty"` //nolint:tagliatelle
	DbEndpoint  string `yaml:"db-endpoint,omitempty"`  //nolint:tagliatelle
	DbName      string `yaml:"db-name,omitempty"`      //nolint:tagliatelle
	DbUser      string `yaml:"db-user,omitempty"`      //nolint:tagliatelle
	DbPassword  string `yaml:"db-password,omitempty"`  //nolint:tagliatelle
	DbPath      string `yaml:"db-path,omitempty"`      //nolint:tagliatelle
}

const (
	OwnEndpointDefault = "tcp://127.0.0.1:8528"
	DbEndpointDefault  = "127.0.0.1:9000"
	DbNameDefault      = "nil_database"
	DbUserDefault      = "default"
	DbPasswordDefault  = ""
	DbPathDefault      = "indexer.db"
)

func (c *Config) ResetToDefault() {
	c.UseBadger = false
	c.OwnEndpoint = OwnEndpointDefault
	c.DbEndpoint = DbEndpointDefault
	c.DbName = DbNameDefault
	c.DbUser = DbUserDefault
	c.DbPassword = DbPasswordDefault
	c.DbPath = DbPathDefault
}

type Service struct {
	Driver indexerdriver.IndexerDriver
}

type IndexerJsonRpc interface {
	GetAddressActions(
		ctx context.Context,
		address types.Address,
		since types.BlockNumber,
	) ([]indexertypes.AddressAction, error)
}

func (c *Config) InitFromFile(cfgFile string) bool {
	if cfgFile == "" {
		return false
	}
	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		c.ResetToDefault()
		return false
	}
	c.UseBadger = v.GetBool("use-badger")
	c.OwnEndpoint = v.GetString("own-endpoint")
	c.DbEndpoint = v.GetString("db-endpoint")
	c.DbPath = v.GetString("db-path")
	c.DbName = v.GetString("db-name")
	c.DbUser = v.GetString("db-user")
	c.DbPassword = v.GetString("db-password")
	return true
}

func NewService(ctx context.Context, cfg *Config) (*Service, error) {
	s := &Service{}

	var err error
	if cfg.UseBadger {
		s.Driver, err = badger.NewBadgerDriver(cfg.DbPath)
	} else {
		s.Driver, err = clickhouse.NewClickhouseDriver(ctx, cfg.DbEndpoint, cfg.DbUser, cfg.DbPassword, cfg.DbName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create storage driver: %w", err)
	}

	return s, nil
}

func (s *Service) GetAddressActions(
	ctx context.Context,
	address types.Address,
	since types.BlockNumber,
) ([]indexertypes.AddressAction, error) {
	return s.Driver.FetchAddressActions(ctx, address, since)
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
