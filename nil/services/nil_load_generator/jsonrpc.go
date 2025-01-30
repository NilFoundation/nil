package nil_load_generator

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type NilLoadGeneratorAPI interface {
	HealthCheck() bool
	SmartAccountsAddr() []types.Address
	CallSwap(pairShard types.ShardId, amountOut1, amountOut2, swapAmount uint64) (common.Hash, error)
}

type NilLoadGeneratorAPIImpl struct{}

var _ NilLoadGeneratorAPI = (*NilLoadGeneratorAPIImpl)(nil)

func NewNilLoadGeneratorAPI() *NilLoadGeneratorAPIImpl {
	return &NilLoadGeneratorAPIImpl{}
}

func (c NilLoadGeneratorAPIImpl) HealthCheck() bool {
	return true
}

func (c NilLoadGeneratorAPIImpl) SmartAccountsAddr() []types.Address {
	smartAccountsAddr := make([]types.Address, len(smartAccounts))
	for i, smartAccount := range smartAccounts {
		smartAccountsAddr[i] = smartAccount.Addr
	}
	return smartAccountsAddr
}

func (c NilLoadGeneratorAPIImpl) CallSwap(pairShard types.ShardId, amountOut1, amountOut2, swapAmount uint64) (common.Hash, error) {
	if len(pairs) == 0 || len(smartAccounts) == 0 {
		return common.EmptyHash, errors.New("uniswap not initialized yet, please wait")
	}
	if pairShard >= types.ShardId(len(pairs)) {
		return common.EmptyHash, errors.New("invalid pair shard")
	}
	token := []types.Address{types.UsdcFaucetAddress, types.UsdtFaucetAddress}[pairShard%2]
	hash, err := pairs[pairShard].Swap(context.Background(), services[0], client, smartAccounts[0], smartAccounts[0].Addr, big.NewInt(int64(amountOut1)), big.NewInt(int64(amountOut2)), types.NewValueFromUint64(swapAmount), *types.TokenIdForAddress(token))
	if err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}
