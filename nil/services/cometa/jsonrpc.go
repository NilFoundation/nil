package cometa

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

type CometaAPI interface {
	GetContract(ctx context.Context, address types.Address) (*ContractData, error)
	GetLocation(ctx context.Context, address types.Address, pc uint) (*Location, error)
	CompileContract(ctx context.Context, inputJson string) (*ContractData, error)
	DeployContract(ctx context.Context, inputJson string, address types.Address) error
	RegisterContract(ctx context.Context, contractData *ContractData, address types.Address) error
}

type CometaAPIImpl struct {
	ctx     context.Context
	service *Service
	logger  *zerolog.Logger
}

var _ CometaAPI = (*CometaAPIImpl)(nil)

func NewCometaAPI(ctx context.Context, service *Service, logger *zerolog.Logger) *CometaAPIImpl {
	return &CometaAPIImpl{
		ctx:     ctx,
		service: service,
		logger:  logger,
	}
}

func (c *CometaAPIImpl) GetContract(ctx context.Context, address types.Address) (*ContractData, error) {
	res, err := c.service.GetContract(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}
	return res.Data, err
}

func (c *CometaAPIImpl) GetLocation(ctx context.Context, address types.Address, pc uint) (*Location, error) {
	return c.service.GetLocation(ctx, address, pc)
}

func (c *CometaAPIImpl) CompileContract(ctx context.Context, inputJson string) (*ContractData, error) {
	return c.service.CompileContract(inputJson)
}

func (c *CometaAPIImpl) DeployContract(ctx context.Context, inputJson string, address types.Address) error {
	contractData, err := c.service.CompileContract(inputJson)
	if err != nil {
		return fmt.Errorf("failed to compile contract: %w", err)
	}

	if err := c.RegisterContract(ctx, contractData, address); err != nil {
		return fmt.Errorf("failed to register contract: %w", err)
	}
	return err
}

func (c *CometaAPIImpl) RegisterContract(ctx context.Context, contractData *ContractData, address types.Address) error {
	return c.service.RegisterContract(ctx, contractData, address)
}
