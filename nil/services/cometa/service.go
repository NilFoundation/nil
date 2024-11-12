package cometa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/spf13/viper"
)

var logger = logging.NewLogger("cometa")

type Storage interface {
	StoreContract(ctx context.Context, contractData *ContractData, address types.Address) error
	LoadContractData(ctx context.Context, address types.Address) (*ContractData, error)
	LoadContractDataByCodeHash(ctx context.Context, codeHash common.Hash) (*ContractData, error)
	GetAbi(ctx context.Context, address types.Address) (string, error)
}

type CometaJsonRpc interface {
	GetContract(ctx context.Context, address types.Address) (*ContractData, error)
	GetLocation(ctx context.Context, address types.Address, pc uint) (*Location, error)
	GetAbi(ctx context.Context, address types.Address) (string, error)
	GetSourceCode(ctx context.Context, address types.Address) (map[string]string, error)
	CompileContract(ctx context.Context, inputJson string) (*ContractData, error)
	DeployContract(ctx context.Context, inputJson string, address types.Address) error
	RegisterContract(ctx context.Context, contractData *ContractData, address types.Address) error
	GetVersion(ctx context.Context) (string, error)
}

type Service struct {
	storage        Storage
	client         client.Client
	contractsCache *lru.Cache[types.Address, *Contract]
}

var _ CometaJsonRpc = (*Service)(nil)

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
	DbPathDefault       = "cometa.db"
)

func (c *Config) ResetDefualt() {
	c.UseBadger = false
	c.OwnEndpoint = OwnEndpointDefault
	c.NodeEndpoint = NodeEndpointDefault
	c.DbEndpoint = DbEndpointDefault
	c.DbName = DbNameDefault
	c.DbUser = DbUserDefault
	c.DbPassword = DbPasswordDefault
	c.DbPath = DbPathDefault
}

func (c *Config) InitFromFile(cfgFile string) {
	if cfgFile == "" {
		return
	}
	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		logger.Error().Err(err).Msg("failed to read config file")
		c.ResetDefualt()
	} else {
		c.UseBadger = v.GetBool("use-badger")
		c.OwnEndpoint = v.GetString("own-endpoint")
		c.NodeEndpoint = v.GetString("node-endpoint")
		c.DbEndpoint = v.GetString("db-endpoint")
		c.DbPath = v.GetString("db-path")
		c.DbName = v.GetString("db-name")
		c.DbUser = v.GetString("db-user")
		c.DbPassword = v.GetString("db-password")
	}
}

func NewService(ctx context.Context, cfg *Config, client client.Client) (*Service, error) {
	c := &Service{}
	var err error
	if cfg.UseBadger {
		if c.storage, err = NewStorageBadger(cfg); err != nil {
			return nil, fmt.Errorf("failed to create storage: %w", err)
		}
	} else {
		if c.storage, err = NewStorageClick(ctx, cfg); err != nil {
			return nil, fmt.Errorf("failed to create storage: %w", err)
		}
	}
	c.client = client
	c.contractsCache, err = lru.New[types.Address, *Contract](100)
	if err != nil {
		return nil, fmt.Errorf("failed to create contractsCache: %w", err)
	}
	return c, nil
}

func (s *Service) Run(ctx context.Context, cfg *Config) error {
	return s.startRpcServer(ctx, cfg.OwnEndpoint)
}

func (s *Service) RegisterContract(ctx context.Context, contractData *ContractData, address types.Address) error {
	logger.Info().Msg("Deploy contract...")
	code, err := s.client.GetCode(address, "latest")
	if err != nil {
		return fmt.Errorf("failed to get code: %w", err)
	}
	if len(code) == 0 {
		return fmt.Errorf("contract does not exist at address %s", address)
	}

	if !bytes.Equal(code, contractData.Code) {
		return errors.New("compiled bytecode is not equal to the deployed one")
	}

	if err = s.storage.StoreContract(ctx, contractData, address); err != nil {
		return err
	}

	logger.Info().Msg("Contract has been deployed.")

	return nil
}

func (s *Service) DeployContract(ctx context.Context, inputJson string, address types.Address) error {
	contractData, err := s.CompileContract(ctx, inputJson)
	if err != nil {
		return fmt.Errorf("failed to compile contract: %w", err)
	}

	if err := s.RegisterContract(ctx, contractData, address); err != nil {
		return fmt.Errorf("failed to register contract: %w", err)
	}
	return err
}

func (s *Service) CompileContract(ctx context.Context, inputJson string) (*ContractData, error) {
	return CompileJson(inputJson)
}

func (s *Service) GetContract(ctx context.Context, address types.Address) (*ContractData, error) {
	contract, err := s.GetContractControl(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}
	return contract.Data, err
}

func (s *Service) GetContractControl(ctx context.Context, address types.Address) (*Contract, error) {
	contract, ok := s.contractsCache.Get(address)
	if !ok {
		data, err := s.storage.LoadContractData(ctx, address)
		if err != nil {
			if data, err = s.FindContractWithSameCode(ctx, address); err != nil {
				return nil, fmt.Errorf("failed to load contract data: %w", err)
			}
			if err = s.storage.StoreContract(ctx, data, address); err != nil {
				logger.Error().Err(err).Msg("failed to store contract")
			}
			logger.Info().Str("contract", data.Name).Msg("Found twin contract")
		}
		contract, err = NewContractFromData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to create contract from data: %w", err)
		}
	}
	s.contractsCache.Add(address, contract)
	return contract, nil
}

func (s *Service) GetContractAsJson(ctx context.Context, address types.Address) (string, error) {
	contract, err := s.GetContractControl(ctx, address)
	if err != nil {
		return "", err
	}
	res, err := json.Marshal(contract.Data)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (s *Service) FindContractWithSameCode(ctx context.Context, address types.Address) (*ContractData, error) {
	code, err := s.client.GetCode(address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get code: %w", err)
	}
	contractData, err := s.storage.LoadContractDataByCodeHash(ctx, code.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get contract by code hash: %w", err)
	}
	if !bytes.Equal(contractData.Code, code) {
		return nil, errors.New("contract not found")
	}
	return contractData, nil
}

func (s *Service) GetLocation(ctx context.Context, address types.Address, pc uint) (*Location, error) {
	contract, err := s.GetContractControl(ctx, address)
	if err != nil {
		return nil, err
	}
	return contract.GetLocation(pc)
}

func (s *Service) GetAbi(ctx context.Context, address types.Address) (string, error) {
	res, err := s.storage.GetAbi(ctx, address)
	if err == nil {
		return res, nil
	}
	contract, err := s.GetContractControl(ctx, address)
	if err != nil {
		return "", err
	}
	return contract.Data.Abi, nil
}

func (s *Service) GetSourceCode(ctx context.Context, address types.Address) (map[string]string, error) {
	contract, err := s.GetContractControl(ctx, address)
	if err != nil {
		return nil, err
	}
	return contract.Data.SourceCode, nil
}

func (s *Service) GetSourceCodeForFile(ctx context.Context, address types.Address, fileName string) (string, error) {
	sourceCode, err := s.GetSourceCode(ctx, address)
	if err != nil {
		return "", err
	}
	source, ok := sourceCode[fileName]
	if !ok {
		return "", errors.New("file not found")
	}
	return source, nil
}

func (s *Service) GetVersion(ctx context.Context) (string, error) {
	if version.HasGitInfo() {
		return fmt.Sprintf("no-date(%s)", version.GetVersionInfo().GitCommit), nil
	}
	if time, gitCommit, err := version.ParseBuildInfo(); err == nil {
		return fmt.Sprintf("%s(%s)", time, gitCommit), nil
	}
	return "", errors.New("failed to get version")
}

func (s *Service) GetRpcApi() transport.API {
	return transport.API{
		Namespace: "cometa",
		Public:    true,
		Service:   CometaJsonRpc(s),
		Version:   "1.0",
	}
}

func (s *Service) startRpcServer(ctx context.Context, endpoint string) error {
	logger := logging.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		Enabled:         true,
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
	}

	apiList := []transport.API{s.GetRpcApi()}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}
