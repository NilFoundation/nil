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
	RpcSwapLimit     string
	MintTokenAmount0 string
	MintTokenAmount1 string
	SwapAmount       string
	UniswapAccounts  uint32
	ThresholdAmount  string
	MainKeysPath     string
}

var (
	smartAccounts        []uniswap.SmartAccount
	services             []*cliservice.Service
	uniswapSmartAccounts []uniswap.SmartAccount
	uniswapServices      []*cliservice.Service
	pairs                []*uniswap.Pair
	client               *rpc_client.Client
	isInitialized        atomic.Bool
	rpcSwapLimit         types.Uint256
	mintToken0Amount     types.Uint256
	mintToken1Amount     types.Uint256
	swapAmount           types.Uint256
	thresholdAmount      types.Uint256
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

func initializeSmartAccountsAndServices(ctx context.Context, uniswapAccounts uint32, shardIdList []types.ShardId, client *rpc_client.Client, service *cliservice.Service, faucet *faucet.Client) ([]uniswap.SmartAccount, error) {
	res := make([]uniswap.SmartAccount, len(shardIdList))

	var err error
	for i := range uniswapAccounts {
		uniswapSmartAccounts[i], err = uniswap.NewSmartAccount(service, types.BaseShardId)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize smart account for shard %s: %w", types.BaseShardId, err)
		}
		uniswapServices[i] = cliservice.NewService(ctx, client, uniswapSmartAccounts[i].PrivateKey, faucet)
	}

	for i, shardId := range shardIdList {
		res[i], err = uniswap.NewSmartAccount(service, shardId)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize smart account for shard %s: %w", shardId, err)
		}

		services[i] = cliservice.NewService(ctx, client, res[i].PrivateKey, faucet)
	}

	return res, nil
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

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger, nil)
}

func setDefaultVars(cfg Config, shardIdList int) error {
	uniswapServices = make([]*cliservice.Service, cfg.UniswapAccounts)
	services = make([]*cliservice.Service, shardIdList)
	uniswapSmartAccounts = make([]uniswap.SmartAccount, cfg.UniswapAccounts)
	if err := mintToken0Amount.Set(cfg.MintTokenAmount0); err != nil {
		return err
	}
	if err := mintToken1Amount.Set(cfg.MintTokenAmount1); err != nil {
		return err
	}
	if err := swapAmount.Set(cfg.SwapAmount); err != nil {
		return err
	}
	if err := thresholdAmount.Set(cfg.ThresholdAmount); err != nil {
		return err
	}
	return nil
}

func selectPairAndAccount(shardIdList []types.ShardId) ([]types.ShardId, []types.ShardId, error) {
	numberCalls, err := rand.Int(rand.Reader, big.NewInt(int64(len(shardIdList)+1)))
	if err != nil {
		return nil, nil, err
	}
	pairsToCall, err := randomPermutation(shardIdList, numberCalls.Uint64())
	if err != nil {
		return nil, nil, err
	}
	smartAccountsToCall, err := randomPermutation(shardIdList, numberCalls.Uint64())
	if err != nil {
		return nil, nil, err
	}
	return pairsToCall, smartAccountsToCall, nil
}

func mint(ctx context.Context, i int, shardIdList []types.ShardId, logger zerolog.Logger) error {
	token2 := types.EthFaucetAddress
	logger.Info().Msgf("Minting liqudity for smart account %s on shard %v", smartAccounts[i].Addr, shardIdList[i])
	token1 := types.UsdtFaucetAddress
	if i%2 == 0 {
		token1 = types.UsdcFaucetAddress
	}
	return pairs[i].Mint(
		ctx, services[i], client, smartAccounts[i], smartAccounts[i].Addr,
		[]types.TokenBalance{
			{Token: *types.TokenIdForAddress(token1), Balance: types.Value{Uint256: &mintToken0Amount}},
			{Token: *types.TokenIdForAddress(token2), Balance: types.Value{Uint256: &mintToken1Amount}},
		},
	)
}

func swap(ctx context.Context, whoWantSwap, whatPairHeWant types.ShardId, logger zerolog.Logger) error {
	token2 := types.EthFaucetAddress
	token1 := types.UsdtFaucetAddress
	if whatPairHeWant%2 == 0 {
		token1 = types.UsdcFaucetAddress
	}
	reserve0, reserve1, err := pairs[whatPairHeWant].GetReserves(services[whoWantSwap])
	if err != nil {
		return err
	}
	expectedOutputAmount := calculateOutputAmount(swapAmount.ToBig(), reserve0, reserve1)
	logger.Info().Msgf("User: %v, Pair: %v, AmountSend: %d,  AmountGet: %d, TokenFrom: %s, TokenTo %s", whoWantSwap, whatPairHeWant, swapAmount, expectedOutputAmount, token1, token2)

	if _, err = pairs[whatPairHeWant].Swap(ctx, services[whoWantSwap], client, smartAccounts[whoWantSwap], smartAccounts[whoWantSwap].Addr, big.NewInt(0), expectedOutputAmount, types.Value{Uint256: &swapAmount}, *types.TokenIdForAddress(token1)); err != nil {
		return err
	}
	return nil
}

func burn(ctx context.Context, i int, shardIdList []types.ShardId, logger zerolog.Logger) error {
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
}

func deployPairs(ctx context.Context, i int, shardIdList []types.ShardId, logger zerolog.Logger) error {
	contracts, err := compileContracts([]string{"UniswapV2Factory", "Token", "UniswapV2Pair"})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to compile contracts")
		return err
	}
	factories := make([]*uniswap.Factory, len(shardIdList))
	token2 := types.EthFaucetAddress
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

	if err := rpcSwapLimit.Set(cfg.RpcSwapLimit); err != nil {
		return err
	}

	mainPrivateKey, err := execution.LoadKeys(cfg.MainKeysPath)
	if err != nil {
		return err
	}

	service := cliservice.NewService(ctx, client, mainPrivateKey, faucet)
	shardIdList, err := client.GetShardIdList(ctx)
	if err != nil {
		return err
	}
	if err := setDefaultVars(cfg, len(shardIdList)); err != nil {
		return err
	}
	logger.Info().Msg("Creating smart accounts...")
	smartAccounts, err = initializeSmartAccountsAndServices(ctx, cfg.UniswapAccounts, shardIdList, client, service, faucet)
	if err != nil {
		return err
	}
	logger.Info().Msg("Smart accounts created successfully.")

	pairs = make([]*uniswap.Pair, len(shardIdList))
	if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
		return deployPairs(ctx, i, shardIdList, logger)
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
				if err := uniswap.TopUpBalance(thresholdAmount, append(services, uniswapServices...), append(smartAccounts, uniswapSmartAccounts...)); err != nil {
					return err
				}
			}
			checkBalanceCounterDownInt--
			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				return mint(ctx, i, shardIdList, logger)
			}); err != nil {
				logger.Error().Err(err).Msg("Minting error")
				return err
			}

			for range cfg.SwapPerIteration {
				pairsToCall, smartAccountsToCall, err := selectPairAndAccount(shardIdList)
				if err != nil {
					return err
				}
				if err := parallelizeAcrossN(len(pairsToCall), func(i int) error {
					return swap(ctx, smartAccountsToCall[i]-1, pairsToCall[i]-1, logger)
				}); err != nil {
					logger.Error().Err(err).Msg("Swap error")
				}
			}

			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				return burn(ctx, i, shardIdList, logger)
			}); err != nil {
				logger.Error().Err(err).Msg("Burn error")
				return err
			}
			logger.Info().Msg("Iteration finished.")
		}
	}
}
