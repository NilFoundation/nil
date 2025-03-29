package nil_load_generator

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	uniswap "github.com/NilFoundation/nil/nil/services/nil_load_generator/contracts"
)

type NilLoadGeneratorAPI interface {
	HealthCheck() bool
	SmartAccountsAddr() []types.Address
	CallSwap(
		pairs []*uniswap.Pair,
		tokenName1 string, tokenName2 string,
		amountSwap types.Uint256,
		expectedAmount types.Uint256,
	) (common.Hash, error)
	CallQuote(pairs []*uniswap.Pair, tokenName1, tokenName2 string, swapAmount types.Uint256) (types.Uint256, error)
	CallInfo(hash common.Hash) (UniswapTransactionInfo, error)
}

type UniswapTokenInfo struct {
	Addr   types.Address
	Name   string
	Amount string
}
type UniswapTransactionInfo struct {
	External bool
	Shard    types.ShardId
	From     types.Address
	To       types.Address
	Tokens   []UniswapTokenInfo
	Success  bool
	Txs      []UniswapTransactionInfo
	OutTxs   []common.Hash
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

type NilLoadGeneratorAPIImpl struct {
	service *Service
}

var _ NilLoadGeneratorAPI = (*NilLoadGeneratorAPIImpl)(nil)

func NewNilLoadGeneratorAPI(serviceData *Service) *NilLoadGeneratorAPIImpl {
	return &NilLoadGeneratorAPIImpl{service: serviceData}
}

func (c NilLoadGeneratorAPIImpl) HealthCheck() bool {
	return c.service.isInitialized.Load()
}

func (c NilLoadGeneratorAPIImpl) SmartAccountsAddr() []types.Address {
	if !c.service.isInitialized.Load() {
		return nil
	}
	smartAccountsAddr := make([]types.Address, len(c.service.smartAccounts))
	for i, smartAccount := range c.service.smartAccounts {
		smartAccountsAddr[i] = smartAccount.Addr
	}
	return smartAccountsAddr
}

func (c NilLoadGeneratorAPIImpl) CallSwap(
	pairs []*uniswap.Pair,
	tokenName1 string,
	tokenName2 string,
	amountSwap types.Uint256,
	expectedAmount types.Uint256,
) (common.Hash, error) {
	res, ok := AvailablePairs[[2]string{tokenName1, tokenName2}]
	if !ok {
		return common.EmptyHash, errors.New("swap for this pair is not available")
	}
	if !c.service.isInitialized.Load() {
		return common.EmptyHash, errors.New("uniswap not initialized yet, please wait")
	}
	if amountSwap.ToBig().Cmp(c.service.config.RpcSwapLimit.ToBig()) > 0 {
		return common.EmptyHash, errors.New("swap amount should be less")
	}
	amount1 := types.Uint256{0}
	amount2 := expectedAmount
	if res.Address == types.EthFaucetAddress {
		amount2 = types.Uint256{0}
		amount1 = expectedAmount
	}
	uniswapSmartAccount, err := c.service.getRandomSmartAccount()
	if err != nil {
		return common.EmptyHash, err
	}
	calldata, err := pairs[res.ShardId-1].Abi.Pack(
		"swap", amount1, amount2, uniswapSmartAccount.Addr)
	if err != nil {
		return common.EmptyHash, err
	}
	return c.service.client.SendTransactionViaSmartAccount(
		context.Background(),
		uniswapSmartAccount.Addr,
		calldata,
		types.NewFeePackFromGas(0),
		types.NewZeroValue(),
		[]types.TokenBalance{
			{
				Token:   *types.TokenIdForAddress(res.Address),
				Balance: types.Value{Uint256: &amountSwap},
			},
		},
		pairs[res.ShardId-1].Addr,
		uniswapSmartAccount.PrivateKey,
	)
}

func (c NilLoadGeneratorAPIImpl) CallQuote(
	pairs []*uniswap.Pair,
	tokenName1 string,
	tokenName2 string,
	swapAmount types.Uint256,
) (types.Uint256, error) {
	res, ok := AvailablePairs[[2]string{tokenName1, tokenName2}]
	if !ok {
		return types.Uint256{0}, errors.New("quote for this pair is not available")
	}
	if !c.service.isInitialized.Load() {
		return types.Uint256{0}, errors.New("uniswap not initialized yet, please wait")
	}
	uniswapSmartAccount, err := c.service.getRandomSmartAccount()
	if err != nil {
		return types.Uint256{0}, err
	}
	reserve0, reserve1, err := pairs[res.ShardId-1].GetReserves(uniswapSmartAccount)
	if err != nil {
		return types.Uint256{0}, err
	}
	if res.Address == types.EthFaucetAddress {
		reserve0, reserve1 = reserve1, reserve0
	}
	expectedOutputAmount := calculateOutputAmount(swapAmount.ToBig(), reserve0, reserve1)
	var expected types.Uint256
	expected.SetFromBig(expectedOutputAmount)
	return expected, nil
}

func getSwapInfo(hash common.Hash, uniswapService *cliservice.Service) (UniswapTransactionInfo, error) {
	tx, err := uniswapService.FetchTransactionByHash(hash)
	if err != nil {
		return UniswapTransactionInfo{}, err
	}
	if tx == nil {
		return UniswapTransactionInfo{}, errors.New("transaction not found")
	}

	outTxs := make([]common.Hash, 0)
	receipt, err := uniswapService.FetchReceiptByHash(hash)
	if err == nil && receipt != nil {
		outTxs = receipt.OutTransactions
	}
	uniswapTxs := make([]UniswapTransactionInfo, 0, len(outTxs))

	for _, curTx := range outTxs {
		txInfo, err := getSwapInfo(curTx, uniswapService)
		if err != nil {
			continue
		}
		uniswapTxs = append(uniswapTxs, txInfo)
	}

	tokenInfo := make([]UniswapTokenInfo, 0, len(tx.Token))
	for _, token := range tx.Token {
		tokenInfo = append(tokenInfo, UniswapTokenInfo{
			Addr:   types.Address(token.Token),
			Name:   types.GetTokenName(token.Token),
			Amount: token.Balance.String(),
		})
	}
	return UniswapTransactionInfo{
		External: !tx.Flags.IsInternal(),
		Shard:    types.ShardIdFromHash(tx.Hash),
		From:     tx.From,
		To:       tx.To,
		Success:  tx.Success,
		Txs:      uniswapTxs,
		Tx:       tx.Hash,
		OutTxs:   outTxs,
		Block:    tx.BlockNumber,
		Tokens:   tokenInfo,
	}, nil
}

func (c NilLoadGeneratorAPIImpl) CallInfo(hash common.Hash) (UniswapTransactionInfo, error) {
	if !c.service.isInitialized.Load() {
		return UniswapTransactionInfo{}, errors.New("uniswap not initialized yet, please wait")
	}
	uniswapSmartAccountWithCliService, err := c.service.getRandomSmartAccount()
	if err != nil {
		return UniswapTransactionInfo{}, err
	}
	return getSwapInfo(hash, uniswapSmartAccountWithCliService.CliService)
}
