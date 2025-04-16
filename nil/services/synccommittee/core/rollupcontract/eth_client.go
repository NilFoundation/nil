package rollupcontract

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthClient interface {
	bind.ContractBackend
	ChainID(ctx context.Context) (*big.Int, error)
	TransactionByHash(ctx context.Context, hash ethcommon.Hash) (tx *types.Transaction, isPending bool, err error)
	TransactionReceipt(ctx context.Context, txHash ethcommon.Hash) (*types.Receipt, error)
}

func NewRetryingEthClient(
	ctx context.Context, endpoint string, requestsTimeout time.Duration, logger logging.Logger,
) (EthClient, error) {
	ethClient, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("connecting to ETH RPC node: %w", err)
	}

	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.DelayExponential(100*time.Millisecond, time.Second),
		},
		logger,
	)

	return &retryingEthClient{ethClient, retryRunner, requestsTimeout}, nil
}

type retryingEthClient struct {
	c           *ethclient.Client
	retryRunner common.RetryRunner
	timeout     time.Duration
}

var _ EthClient = (*retryingEthClient)(nil)

func (rec *retryingEthClient) CallContract(
	ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int,
) ([]byte, error) {
	return retry2(ctx, rec, func(ctx context.Context) ([]byte, error) {
		return rec.c.CallContract(ctx, msg, blockNumber)
	})
}

func (rec *retryingEthClient) ChainID(ctx context.Context) (*big.Int, error) {
	return retry2(ctx, rec, func(ctx context.Context) (*big.Int, error) {
		return rec.c.ChainID(ctx)
	})
}

func (rec *retryingEthClient) CodeAt(
	ctx context.Context, account ethcommon.Address, blockNumber *big.Int,
) ([]byte, error) {
	return retry2(ctx, rec, func(ctx context.Context) ([]byte, error) {
		return rec.c.CodeAt(ctx, account, blockNumber)
	})
}

func (rec *retryingEthClient) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return retry2(ctx, rec, func(ctx context.Context) (uint64, error) {
		return rec.c.EstimateGas(ctx, msg)
	})
}

func (rec *retryingEthClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return retry2(ctx, rec, func(ctx context.Context) ([]types.Log, error) {
		return rec.c.FilterLogs(ctx, q)
	})
}

func (rec *retryingEthClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return retry2(ctx, rec, func(ctx context.Context) (*types.Header, error) {
		return rec.c.HeaderByNumber(ctx, number)
	})
}

func (rec *retryingEthClient) PendingCodeAt(ctx context.Context, account ethcommon.Address) ([]byte, error) {
	return retry2(ctx, rec, func(ctx context.Context) ([]byte, error) {
		return rec.c.PendingCodeAt(ctx, account)
	})
}

func (rec *retryingEthClient) PendingNonceAt(ctx context.Context, account ethcommon.Address) (uint64, error) {
	return retry2(ctx, rec, func(ctx context.Context) (uint64, error) {
		return rec.c.PendingNonceAt(ctx, account)
	})
}

func (rec *retryingEthClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return retry1(ctx, rec, func(ctx context.Context) error {
		return rec.c.SendTransaction(ctx, tx)
	})
}

func (rec *retryingEthClient) SubscribeFilterLogs(
	ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log,
) (ethereum.Subscription, error) {
	return retry2(ctx, rec, func(ctx context.Context) (ethereum.Subscription, error) {
		return rec.c.SubscribeFilterLogs(ctx, q, ch)
	})
}

func (rec *retryingEthClient) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return retry2(ctx, rec, func(ctx context.Context) (*big.Int, error) {
		return rec.c.SuggestGasPrice(ctx)
	})
}

func (rec *retryingEthClient) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return retry2(ctx, rec, func(ctx context.Context) (*big.Int, error) {
		return rec.c.SuggestGasTipCap(ctx)
	})
}

func (rec *retryingEthClient) TransactionReceipt(ctx context.Context, txHash ethcommon.Hash) (*types.Receipt, error) {
	return retry2(ctx, rec, func(ctx context.Context) (*types.Receipt, error) {
		return rec.c.TransactionReceipt(ctx, txHash)
	})
}

func (rec *retryingEthClient) TransactionByHash(
	ctx context.Context, hash ethcommon.Hash,
) (tx *types.Transaction, isPending bool, err error) {
	return retry3(ctx, rec, func(ctx context.Context) (*types.Transaction, bool, error) {
		return rec.c.TransactionByHash(ctx, hash)
	})
}

// retry3 is a generic retry helper for any function that returns (T1, T2, error)
func retry3[T1 any, T2 any](
	ctx context.Context, rec *retryingEthClient, fn func(context.Context,
	) (T1, T2, error),
) (T1, T2, error) {
	var ret1 T1
	var ret2 T2
	err := rec.retryRunner.Do(ctx, func(ctx context.Context) error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, rec.timeout)
		defer cancel()
		var err error
		ret1, ret2, err = fn(ctxWithTimeout)
		return err
	})
	return ret1, ret2, err
}

// retry2 is a generic retry helper for any function that returns (T, error)
func retry2[T any](ctx context.Context, rec *retryingEthClient, fn func(context.Context) (T, error)) (T, error) {
	var ret T
	err := rec.retryRunner.Do(ctx, func(ctx context.Context) error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, rec.timeout)
		defer cancel()
		var err error
		ret, err = fn(ctxWithTimeout)
		return err
	})
	return ret, err
}

// retry1 a retry helper for any function that returns (error)
func retry1(ctx context.Context, rec *retryingEthClient, fn func(context.Context) error) error {
	return rec.retryRunner.Do(ctx, func(ctx context.Context) error {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, rec.timeout)
		defer cancel()
		return fn(ctxWithTimeout)
	})
}
