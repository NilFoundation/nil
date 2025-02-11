package nil_load_generator

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os/signal"
	"sync/atomic"
	"syscall"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/faucet"
	uniswap "github.com/NilFoundation/nil/nil/services/nil_load_generator/contracts"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Endpoint         string
	OwnEndpoint      string
	FaucetEndpoint   string
	CheckBalance     uint32
	SwapPerIteration uint32
	Metrics          bool
	LogLevel         string
}

const (
	mintToken0Amount = 10000
	mintToken1Amount = 10000
	swapAmount       = 1000
)

var (
	smartAccounts       []uniswap.SmartAccount
	services            []*cliservice.Service
	uniswapSmartAccount uniswap.SmartAccount
	uniswapService      *cliservice.Service
	pairs               []*uniswap.Pair
	client              *rpc_client.Client
	isInitialized       atomic.Bool
)

func calculateOutputAmount(amountIn, reserveIn, reserveOut *big.Int) *big.Int {
	feeMultiplier := big.NewInt(997)
	feeDivisor := big.NewInt(1000)

	amountInWithFee := new(big.Int).Mul(amountIn, feeMultiplier)
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Mul(reserveIn, feeDivisor)
	denominator.Add(denominator, amountInWithFee)
	outputAmount := new(big.Int).Div(numerator, denominator)
	return outputAmount
}

func randomPermutation(shardIdList []types.ShardId, amount uint64) ([]types.ShardId, error) {
	arr := shardIdList
	for i := len(arr) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		j := jBig.Uint64()
		arr[i], arr[j] = arr[j], arr[i]
	}
	return arr[:amount], nil
}

func initializeSmartAccountsAndServices(ctx context.Context, shardIdList []types.ShardId, client *rpc_client.Client, service *cliservice.Service, faucet *faucet.Client) ([]uniswap.SmartAccount, []*cliservice.Service, error) {
	res := make([]uniswap.SmartAccount, len(shardIdList))
	services := make([]*cliservice.Service, len(shardIdList))

	var err error
	uniswapSmartAccount, err = uniswap.NewSmartAccount(service, types.BaseShardId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize smart account for shard %s: %w", types.BaseShardId, err)
	}
	uniswapService = cliservice.NewService(ctx, client, uniswapSmartAccount.PrivateKey, faucet)

	for i, shardId := range shardIdList {
		res[i], err = uniswap.NewSmartAccount(service, shardId)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize smart account for shard %s: %w", shardId, err)
		}

		services[i] = cliservice.NewService(ctx, client, res[i].PrivateKey, faucet)
	}

	return res, services, nil
}

func compileContracts(contractNames []string) (map[string]uniswap.Contract, error) {
	contractsRes := make(map[string]uniswap.Contract)
	for _, name := range contractNames {
		code, err := contracts.GetCode("uniswap/" + name)
		if err != nil {
			return nil, fmt.Errorf("failed to get code for contract %s: %w", name, err)
		}
		abi, err := contracts.GetAbi("uniswap/" + name)
		if err != nil {
			return nil, fmt.Errorf("failed to get abi for contract %s: %w", name, err)
		}
		contractsRes[name] = uniswap.Contract{Abi: *abi, Code: code}
	}
	return contractsRes, nil
}

func parallelizeAcrossN(n int, task func(i int) error) error {
	var g errgroup.Group

	for i := range n {
		g.Go(func() error {
			return task(i)
		})
	}
	return g.Wait()
}

func startRpcServer(ctx context.Context, endpoint string) error {
	logger := logging.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
	}

	nilLoadGeneratorApi := NewNilLoadGeneratorAPI()

	apiList := []transport.API{
		{
			Namespace: "nilloadgen",
			Public:    true,
			Service:   NilLoadGeneratorAPI(nilLoadGeneratorApi),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}

func Run(ctx context.Context, cfg Config, logger zerolog.Logger) error {
	signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	ctx = signalCtx

	go func() {
		if err := startRpcServer(ctx, cfg.OwnEndpoint); err != nil {
			logger.Error().Err(err).Msg("Failed to start RPC server")
			panic(err)
		}
	}()

	faucet := faucet.NewClient(cfg.FaucetEndpoint)
	client = rpc_client.NewClient(cfg.Endpoint, logger)
	logging.SetupGlobalLogger(cfg.LogLevel)

	service := cliservice.NewService(ctx, client, execution.MainPrivateKey, faucet)
	shardIdList, err := client.GetShardIdList(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get shard id list")
		return err
	}
	logger.Info().Msg("Creating smart accounts...")
	smartAccounts, services, err = initializeSmartAccountsAndServices(ctx, shardIdList, client, service, faucet)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize smart accounts and services")
		return err
	}

	logger.Info().Msg("Smart accounts created successfully.")
	contractNames := []string{"UniswapV2Factory", "Token", "UniswapV2Pair", "UniswapV2Router01"}
	contracts, err := compileContracts(contractNames)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to compile contracts")
		return err
	}

	factories := make([]*uniswap.Factory, len(shardIdList))
	pairs = make([]*uniswap.Pair, len(shardIdList))
	token2 := types.EthFaucetAddress

	if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
		token1 := types.UsdtFaucetAddress
		if i%2 == 0 {
			token1 = types.UsdcFaucetAddress
		}

		logger.Info().Msgf("Deploying factory on shard %v", shardIdList[i])
		factories[i] = uniswap.NewFactory(contracts["UniswapV2Factory"])
		if err := factories[i].Deploy(services[i], smartAccounts[i], smartAccounts[i].Addr); err != nil {
			return fmt.Errorf("failed to deploy factory on shard %v: %w", shardIdList[i], err)
		}

		logger.Info().Msgf("Creating pair on shard %v", shardIdList[i])
		if err := factories[i].CreatePair(ctx, services[i], client, smartAccounts[i], token1, token2); err != nil {
			return fmt.Errorf("failed to create pair on shard %v: %w", shardIdList[i], err)
		}

		logger.Info().Msgf("Initializing pair on shard %v", shardIdList[i])
		pairAddress, err := factories[i].GetPair(services[i], token1, token2)
		if err != nil {
			return fmt.Errorf("failed to get pair on shard %v: %w", shardIdList[i], err)
		}

		pairs[i] = uniswap.NewPair(contracts["UniswapV2Pair"], pairAddress)
		if err := pairs[i].Initialize(ctx, services[i], client, smartAccounts[i], token1, token2); err != nil {
			return fmt.Errorf("failed to initialize pair on shard %v: %w", shardIdList[i], err)
		}

		return nil
	}); err != nil {
		logger.Error().Err(err).Msg("Deployment and initialization error")
		return err
	}
	isInitialized.Store(true)
	logger.Info().Msg("Starting main loop.")
	checkBalanceCounterDownInt := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if checkBalanceCounterDownInt == 0 {
				checkBalanceCounterDownInt = int(cfg.CheckBalance)
				logger.Info().Msg("Checking balance and minting tokens.")
				err := uniswap.TopUpBalance(append(services, uniswapService), append(smartAccounts, uniswapSmartAccount))
				if err != nil {
					return err
				}
			}
			checkBalanceCounterDownInt--
			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				logger.Info().Msgf("Minting liqudity for smart account %s on shard %v", smartAccounts[i].Addr, shardIdList[i])
				token1 := types.UsdtFaucetAddress
				if i%2 == 0 {
					token1 = types.UsdcFaucetAddress
				}
				return pairs[i].Mint(
					ctx, services[i], client, smartAccounts[i], smartAccounts[i].Addr,
					[]types.TokenBalance{
						{Token: *types.TokenIdForAddress(token1), Balance: types.NewValueFromUint64(mintToken0Amount)},
						{Token: *types.TokenIdForAddress(token2), Balance: types.NewValueFromUint64(mintToken1Amount)},
					},
				)
			}); err != nil {
				logger.Error().Err(err).Msg("Minting error")
				return err
			}

			for range cfg.SwapPerIteration {
				numberCalls, err := rand.Int(rand.Reader, big.NewInt(int64(len(shardIdList)+1)))
				if err != nil {
					return err
				}
				pairsToCall, err := randomPermutation(shardIdList, numberCalls.Uint64())
				if err != nil {
					return err
				}
				smartAccountsToCall, err := randomPermutation(shardIdList, numberCalls.Uint64())
				if err != nil {
					return err
				}
				if err := parallelizeAcrossN(int(numberCalls.Int64()), func(i int) error {
					token1 := types.UsdtFaucetAddress
					if i%2 == 0 {
						token1 = types.UsdcFaucetAddress
					}
					whoWantSwap := smartAccountsToCall[i] - 1
					whatPairHeWant := pairsToCall[i] - 1
					reserve0, reserve1, err := pairs[whatPairHeWant].GetReserves(services[whoWantSwap])
					if err != nil {
						return err
					}
					expectedOutputAmount := calculateOutputAmount(big.NewInt(swapAmount), reserve0, reserve1)
					logger.Info().Msgf("User: %v, Pair: %v, AmountSend: %d,  AmountGet: %d, TokenFrom: %s, TokenTo %s", whoWantSwap, whatPairHeWant, swapAmount, expectedOutputAmount, token1, token2)

					if _, err = pairs[whatPairHeWant].Swap(ctx, services[whoWantSwap], client, smartAccounts[whoWantSwap], smartAccounts[whoWantSwap].Addr, big.NewInt(0), expectedOutputAmount, types.NewValueFromUint64(swapAmount), *types.TokenIdForAddress(token1)); err != nil {
						return err
					}
					return nil
				}); err != nil {
					logger.Error().Err(err).Msg("Minting error")
					return err
				}
			}

			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				logger.Info().Msgf("Burn liquidity for user smart account %s on shard %v", smartAccounts[i].Addr, shardIdList[i])
				userLpBalance, err := pairs[i].GetTokenBalanceOf(services[i], smartAccounts[i].Addr)
				if err != nil {
					return err
				}
				if userLpBalance.Uint64() > 0 {
					return pairs[i].Burn(
						ctx, services[i], client, smartAccounts[i], smartAccounts[i].Addr,
						types.TokenId(pairs[i].Addr),
						types.NewValueFromUint64(userLpBalance.Uint64()),
					)
				}
				return nil
			}); err != nil {
				logger.Error().Err(err).Msg("Burn error")
				return err
			}
			logger.Info().Msg("Iteration finished.")
		}
	}
}
