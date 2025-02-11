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
	CallSwap(tokenName1, tokenName2 string, amountIn, swapAmount uint64) (common.Hash, error)
	CallQuote(tokenName1, tokenName2 string, swapAmount uint64) (*big.Int, error)
	CallInfo(hash common.Hash) (UniswapTransactionInfo, error)
}

type UniswapTokenInfo struct {
	Addr   types.Address
	Name   string
	Amount uint64
}
type UniswapTransactionInfo struct {
	External bool
	Shard    types.ShardId
	From     types.Address
	To       types.Address
	Tokens   UniswapTokenInfo
	Success  bool
	Txs      []common.Hash
	Tx       common.Hash
	Block    types.BlockNumber
}

var AvailablePairs = map[[2]string]struct {
	ShardId types.ShardId
	Address types.Address
}{
	{"USDT", "ETH"}: {types.ShardId(2), types.UsdtFaucetAddress},
	{"ETH", "USDT"}: {types.ShardId(2), types.EthFaucetAddress},
	{"USDC", "ETH"}: {types.ShardId(1), types.UsdcFaucetAddress},
	{"ETH", "USDC"}: {types.ShardId(1), types.EthFaucetAddress},
}

type NilLoadGeneratorAPIImpl struct{}

var _ NilLoadGeneratorAPI = (*NilLoadGeneratorAPIImpl)(nil)

func NewNilLoadGeneratorAPI() *NilLoadGeneratorAPIImpl {
	return &NilLoadGeneratorAPIImpl{}
}

func (c NilLoadGeneratorAPIImpl) HealthCheck() bool {
	return isInitialized.Load()
}

func (c NilLoadGeneratorAPIImpl) SmartAccountsAddr() []types.Address {
	if !isInitialized.Load() {
		return nil
	}
	smartAccountsAddr := make([]types.Address, len(smartAccounts))
	for i, smartAccount := range smartAccounts {
		smartAccountsAddr[i] = smartAccount.Addr
	}
	return smartAccountsAddr
}

func (c NilLoadGeneratorAPIImpl) CallSwap(tokenName1, tokenName2 string, amountIn, swapAmount uint64) (common.Hash, error) {
	res, ok := AvailablePairs[[2]string{tokenName1, tokenName2}]
	if !ok {
		return common.EmptyHash, errors.New("Quote for this pair is not available")
	}
	if !isInitialized.Load() {
		return common.EmptyHash, errors.New("uniswap not initialized yet, please wait")
	}
	calldata, err := pairs[res.ShardId-1].Abi.Pack("swap", big.NewInt(int64(0)), big.NewInt(int64(amountIn)), uniswapSmartAccount.Addr)
	if err != nil {
		return common.EmptyHash, err
	}
	return client.SendTransactionViaSmartAccount(context.Background(), uniswapSmartAccount.Addr, calldata,
		types.NewZeroValue(), types.NewZeroValue(), []types.TokenBalance{
			{Token: *types.TokenIdForAddress(res.Address), Balance: types.NewValueFromUint64(swapAmount)}}, pairs[res.ShardId-1].Addr, uniswapSmartAccount.PrivateKey)
}

func (c NilLoadGeneratorAPIImpl) CallQuote(tokenName1, tokenName2 string, swapAmount uint64) (*big.Int, error) {
	res, ok := AvailablePairs[[2]string{tokenName1, tokenName2}]
	if !ok {
		return big.NewInt(0), errors.New("Quote for this pair is not available")
	}
	if !isInitialized.Load() {
		return big.NewInt(0), errors.New("uniswap not initialized yet, please wait")
	}
	reserve0, reserve1, err := pairs[res.ShardId-1].GetReserves(uniswapService)
	if err != nil {
		return big.NewInt(0), err
	}
	if res.Address == types.EthFaucetAddress {
		reserve0, reserve1 = reserve1, reserve0
	}
	expectedOutputAmount := calculateOutputAmount(big.NewInt(int64(swapAmount)), reserve0, reserve1)
	return expectedOutputAmount, nil
}

func (c NilLoadGeneratorAPIImpl) CallInfo(hash common.Hash) (UniswapTransactionInfo, error) {
	if !isInitialized.Load() {
		return UniswapTransactionInfo{}, errors.New("uniswap not initialized yet, please wait")
	}
	tx, err := uniswapService.FetchTransactionByHash(hash)
	if err != nil {
		return UniswapTransactionInfo{}, err
	}
	outTxs := make([]common.Hash, 0)
	receipt, err := uniswapService.FetchReceiptByHash(hash)
	if err == nil && receipt != nil {
		outTxs = receipt.OutTransactions
	}
	return UniswapTransactionInfo{
		External: !(tx.Flags == types.NewTransactionFlags(types.TransactionFlagInternal)),
		Shard:    types.ShardIdFromHash(tx.Hash),
		From:     tx.From,
		To:       tx.To,
		Success:  tx.Success,
		Txs:      outTxs,
		Tx:       tx.Hash,
		Block:    tx.BlockNumber,
		Tokens:   UniswapTokenInfo{},
	}, nil
}
