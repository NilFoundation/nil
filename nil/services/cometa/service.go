package cometa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	lru "github.com/hashicorp/golang-lru/v2"
)

var logger = logging.NewLogger("cometa")

type Service struct {
	storage        *Storage
	client         client.Client
	contractsCache *lru.Cache[types.Address, *Contract]
}

func NewService(dbPath string, client client.Client) (*Service, error) {
	c := &Service{}
	var err error
	if c.storage, err = NewStorage(dbPath); err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	c.client = client
	c.contractsCache, err = lru.New[types.Address, *Contract](100)
	if err != nil {
		return nil, fmt.Errorf("failed to create contractsCache: %w", err)
	}
	return c, nil
}

func (s *Service) Run(ctx context.Context, endpoint string) error {
	err := s.startRpcServer(ctx, endpoint)
	return err
}

func (s *Service) RegisterContract(ctx context.Context, contractData *ContractData, address types.Address) error {
	logger.Info().Msg("Deploy contract...")
	code, err := s.client.GetCode(address, "latest")
	if err != nil {
		return fmt.Errorf("failed to get code: %w", err)
	}
	if len(code) == 0 {
		return errors.New("contract has no code")
	}
	codeHash := code.Hash()
	compiledCode := types.Code(contractData.Code)

	compiledHash := compiledCode.Hash()
	if codeHash != compiledHash {
		return errors.New("contracts hash mismatch")
	}

	if err = s.storage.StoreContract(ctx, contractData, address); err != nil {
		return err
	}

	logger.Info().Msg("Contract has been deployed.")

	return nil
}

func (s *Service) CompileContract(inputJson string) (*ContractData, error) {
	return Compile(inputJson)
}

func (s *Service) GetContract(ctx context.Context, address types.Address) (*Contract, error) {
	contract, ok := s.contractsCache.Get(address)
	if !ok {
		data, err := s.storage.LoadContractData(ctx, address)
		if err != nil {
			return nil, fmt.Errorf("failed to load contract data: %w", err)
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
	contract, err := s.GetContract(ctx, address)
	if err != nil {
		return "", err
	}
	res, err := json.Marshal(contract.Data)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (s *Service) GetLocation(ctx context.Context, address types.Address, pc uint) (*Location, error) {
	contract, err := s.GetContract(ctx, address)
	if err != nil {
		return nil, err
	}
	return contract.GetLocation(pc)
}

func (s *Service) GetSourceCode(ctx context.Context, address types.Address, fileName string) (string, error) {
	contract, err := s.GetContract(ctx, address)
	if err != nil {
		return "", err
	}
	source, ok := contract.Data.SourceCode[fileName]
	if !ok {
		return "", errors.New("file not found")
	}
	return source, nil
}

func (s *Service) startRpcServer(ctx context.Context, endpoint string) error {
	logger := logging.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		Enabled:         true,
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	cometaApi := NewCometaAPI(ctx, s, &logger)

	apiList := []transport.API{
		{
			Namespace: "cometa",
			Public:    true,
			Service:   CometaAPI(cometaApi),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}
