package nil_load_generator

import (
	"github.com/NilFoundation/nil/nil/internal/types"
)

type NilLoadGeneratorAPI interface {
	HealthCheck() bool
	WalletsAddr() []types.Address
}

type NilLoadGeneratorAPIImpl struct{}

var _ NilLoadGeneratorAPI = (*NilLoadGeneratorAPIImpl)(nil)

func NewNilLoadGeneratorAPI() *NilLoadGeneratorAPIImpl {
	return &NilLoadGeneratorAPIImpl{}
}

func (c NilLoadGeneratorAPIImpl) HealthCheck() bool {
	return true
}

func (c NilLoadGeneratorAPIImpl) WalletsAddr() []types.Address {
	walletsAddr := make([]types.Address, len(wallets))
	for i, wallet := range wallets {
		walletsAddr[i] = wallet.Addr
	}
	return walletsAddr
}
