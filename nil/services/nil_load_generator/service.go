package nil_load_generator

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os/signal"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/faucet"
	uniswap "github.com/NilFoundation/nil/nil/services/nil_load_generator/contracts"
	"github.com/NilFoundation/nil/nil/services/nil_load_generator/metrics"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Endpoint           string
	OwnEndpoint        string
	FaucetEndpoint     string
	CheckBalance       uint32
	SwapPerIteration   uint32
	Metrics            bool
	LogLevel           string
	RpcSwapLimit       types.Uint256
	MintTokenAmount0   types.Value
	MintTokenAmount1   types.Value
	SwapAmount         types.Value
	UniswapAccounts    uint32
	ThresholdAmount    types.Value
	WaitClusterStartup time.Duration
}

func NewDefaultConfig() *Config {
	return &Config{
		Endpoint:           "http://127.0.0.1:8529/",
		OwnEndpoint:        "tcp://127.0.0.1:8525",
		FaucetEndpoint:     "tcp://127.0.0.1:8527",
		CheckBalance:       10,
		SwapPerIteration:   1000,
		Metrics:            false,
		LogLevel:           "info",
		RpcSwapLimit:       *types.NewUint256(1000000),
		MintTokenAmount0:   types.NewValueFromUint64(3000000000000000),
		MintTokenAmount1:   types.NewValueFromUint64(10000000000000),
		SwapAmount:         types.NewValueFromUint64(1000),
		UniswapAccounts:    5,
		ThresholdAmount:    types.NewValueFromUint64(3000000000000000000),
		WaitClusterStartup: 5 * time.Minute,
	}
}

type Service struct {
	isInitialized atomic.Bool

	config *Config
	logger logging.Logger

	shardIdList []types.ShardId

	smartAccounts        []uniswap.SmartAccount
	uniswapSmartAccounts []uniswap.SmartAccount

	client *rpc_client.Client
}

func newService(config *Config, logger logging.Logger) *Service {
	client := rpc_client.NewClient(config.Endpoint, logger)
	return &Service{
		config: config,
		logger: logger,
		client: client,
	}
}

func (s *Service) getRandomSmartAccount() (uniswap.SmartAccount, error) {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(s.uniswapSmartAccounts))))
	if err != nil {
		return uniswap.SmartAccount{}, err
	}
	return s.uniswapSmartAccounts[index.Int64()], nil
}

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
	arr := slices.Clone(shardIdList)
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

func (s *Service) init(ctx context.Context, cfg *Config, shardIdList []types.ShardId) error {
	s.shardIdList = slices.Clone(shardIdList)
	s.smartAccounts = make([]uniswap.SmartAccount, len(shardIdList))
	s.uniswapSmartAccounts = make([]uniswap.SmartAccount, cfg.UniswapAccounts)

	faucet := faucet.NewClient(s.config.FaucetEndpoint)
	cliService := cliservice.NewService(ctx, s.client, nil, faucet)
	var err error
	for i := range cfg.UniswapAccounts {
		s.uniswapSmartAccounts[i], err = uniswap.NewSmartAccount(
			cliService, types.BaseShardId)
		if err != nil {
			return fmt.Errorf("failed to initialize smart account for shard %s: %w", types.BaseShardId, err)
		}
	}

	for i, shardId := range shardIdList {
		s.smartAccounts[i], err = uniswap.NewSmartAccount(cliService, shardId)
		if err != nil {
			return fmt.Errorf("failed to initialize smart account for shard %s: %w", shardId, err)
		}
	}

	return nil
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

func (s *Service) parallelizeAcrossShards(task func(i int) error) error {
	return parallelizeAcrossN(len(s.shardIdList), task)
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

func startRpcServer(ctx context.Context, endpoint string, service *Service) error {
	logger := logging.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         endpoint,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
	}

	nilLoadGeneratorApi := NewNilLoadGeneratorAPI(service)

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

func (s *Service) selectPairAndAccount() ([]types.ShardId, []types.ShardId, error) {
	numberCalls, err := rand.Int(rand.Reader, big.NewInt(int64(len(s.shardIdList)+1)))
	if err != nil {
		return nil, nil, err
	}
	pairsToCall, err := randomPermutation(s.shardIdList, numberCalls.Uint64())
	if err != nil {
		return nil, nil, err
	}
	smartAccountsToCall, err := randomPermutation(s.shardIdList, numberCalls.Uint64())
	if err != nil {
		return nil, nil, err
	}
	return pairsToCall, smartAccountsToCall, nil
}

func (s *Service) mint(ctx context.Context, pairs []*uniswap.Pair, i int) error {
	token2 := types.EthFaucetAddress
	smartAccount := s.smartAccounts[i]
	s.logger.Info().Msgf(
		"Minting liqudity for smart account %s on shard %v",
		smartAccount.Addr, s.shardIdList[i])
	token1 := types.UsdtFaucetAddress
	if i%2 == 0 {
		token1 = types.UsdcFaucetAddress
	}
	return pairs[i].Mint(
		ctx,
		s.smartAccounts[i],
		[]types.TokenBalance{
			{Token: *types.TokenIdForAddress(token1), Balance: s.config.MintTokenAmount0},
			{Token: *types.TokenIdForAddress(token2), Balance: s.config.MintTokenAmount1},
		},
	)
}

func (s *Service) swap(
	ctx context.Context,
	pairs []*uniswap.Pair,
	whoWantSwap types.ShardId,
	whatPairHeWant types.ShardId,
) error {
	token2 := types.EthFaucetAddress
	token1 := types.UsdtFaucetAddress
	if whatPairHeWant%2 == 0 {
		token1 = types.UsdcFaucetAddress
	}
	smartAccount := s.smartAccounts[whoWantSwap]
	reserve0, reserve1, err := pairs[whatPairHeWant].GetReserves(smartAccount)
	if err != nil {
		return err
	}
	expectedOutputAmount := calculateOutputAmount(s.config.SwapAmount.ToBig(), reserve0, reserve1)
	s.logger.Info().Msgf(
		"User: %v, Pair: %v, AmountSend: %s,  AmountGet: %s, TokenFrom: %s, TokenTo %s",
		whoWantSwap, whatPairHeWant, s.config.SwapAmount, expectedOutputAmount, token1, token2)

	if _, err = pairs[whatPairHeWant].Swap(
		ctx,
		smartAccount,
		big.NewInt(0),
		expectedOutputAmount,
		s.config.SwapAmount,
		*types.TokenIdForAddress(token1)); err != nil {
		return err
	}
	return nil
}

func (s *Service) burn(ctx context.Context, pairs []*uniswap.Pair, i int) error {
	smartAccount := s.smartAccounts[i]
	s.logger.Info().Msgf(
		"Burn liquidity for user smart account %s on shard %v",
		smartAccount.Addr, s.shardIdList[i])
	userLpBalance, err := pairs[i].GetTokenBalanceOf(smartAccount)
	if err != nil {
		return err
	}
	if userLpBalance.Uint64() > 0 {
		return pairs[i].Burn(
			ctx,
			smartAccount,
			types.TokenId(pairs[i].Addr),
			types.NewValueFromUint64(userLpBalance.Uint64()),
		)
	}
	return nil
}

func (s *Service) deployPair(ctx context.Context, i int) (*uniswap.Pair, error) {
	contracts, err := compileContracts([]string{"UniswapV2Factory", "Token", "UniswapV2Pair"})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to compile contracts")
		return nil, err
	}

	factories := make([]*uniswap.Factory, len(s.shardIdList))
	token2 := types.EthFaucetAddress
	token1 := types.UsdtFaucetAddress
	if i%2 == 0 {
		token1 = types.UsdcFaucetAddress
	}

	smartAccount := s.smartAccounts[i]
	s.logger.Info().Msgf("Deploying factory on shard %v", s.shardIdList[i])
	factories[i] = uniswap.NewFactory(contracts["UniswapV2Factory"])
	if err := factories[i].Deploy(smartAccount); err != nil {
		return nil, fmt.Errorf("failed to deploy factory on shard %v: %w", s.shardIdList[i], err)
	}

	pairAddress, err := factories[i].GetPair(smartAccount.CliService, token1, token2)
	if err != nil {
		return nil, fmt.Errorf("failed to get pair on shard %v: %w", s.shardIdList[i], err)
	}
	if pairAddress == types.EmptyAddress {
		s.logger.Info().Msgf("Creating pair on shard %v", s.shardIdList[i])
		if err := factories[i].CreatePair(ctx, smartAccount, token1, token2); err != nil {
			return nil, fmt.Errorf("failed to create pair on shard %v: %w", s.shardIdList[i], err)
		}

		s.logger.Info().Msgf("Initializing pair on shard %v", s.shardIdList[i])
		pairAddress, err = factories[i].GetPair(smartAccount.CliService, token1, token2)
		if err != nil {
			return nil, fmt.Errorf("failed to get pair on shard %v: %w", s.shardIdList[i], err)
		}
	}

	pair := uniswap.NewPair(contracts["UniswapV2Pair"], pairAddress)
	if err := pair.Initialize(ctx, smartAccount, token1, token2); err != nil {
		return nil, fmt.Errorf("failed to initialize pair on shard %v: %w", s.shardIdList[i], err)
	}

	return pair, nil
}

func waitClusterStart(
	ctx context.Context,
	timeout time.Duration,
	tick time.Duration,
	client *rpc_client.Client,
) ([]types.ShardId, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			var err error
			shardIdList, err := client.GetShardIdList(ctx)
			if err != nil {
				continue
			}
			allBlockTicking := true
			for _, shardId := range append(shardIdList, types.MainShardId) {
				block, err := client.GetBlock(ctx, shardId, "latest", false)
				if err != nil || block.Number == 0 {
					allBlockTicking = false
					break
				}
			}
			if !allBlockTicking {
				continue
			}
			return shardIdList, nil
		}
	}
}

func Run(ctx context.Context, cfg *Config, logger logging.Logger) error {
	signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	ctx = signalCtx
	handler, err := metrics.NewMetricsHandler("nil_load_generator")
	if err != nil {
		return err
	}

	logging.SetupGlobalLogger(cfg.LogLevel)

	service := newService(cfg, logger)

	go func() {
		if err := startRpcServer(ctx, cfg.OwnEndpoint, service); err != nil {
			handler.RecordError(ctx)
			logger.Error().Err(err).Msg("Failed to start RPC server")
			panic(err)
		}
	}()

	{
		shardIdList, err := waitClusterStart(ctx, cfg.WaitClusterStartup, 5*time.Second, service.client)
		if err != nil {
			handler.RecordError(ctx)
			return err
		}

		logger.Info().Msg("Creating smart accounts...")
		if err = service.init(ctx, cfg, shardIdList); err != nil {
			handler.RecordError(ctx)
			return err
		}
		logger.Info().Msg("Smart accounts created successfully.")
	}

	pairs := make([]*uniswap.Pair, len(service.shardIdList))
	if err := service.parallelizeAcrossShards(func(i int) error {
		var e error
		pairs[i], e = service.deployPair(ctx, i)
		return e
	}); err != nil {
		handler.RecordError(ctx)
		logger.Error().Err(err).Msg("Deployment and initialization error")
		return err
	}
	service.isInitialized.Store(true)
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
				if err := uniswap.TopUpBalance(
					*service.config.ThresholdAmount.Uint256,
					append(service.smartAccounts, service.uniswapSmartAccounts...),
				); err != nil {
					handler.RecordError(ctx)
					return err
				}
			}
			checkBalanceCounterDownInt--
			if err := service.parallelizeAcrossShards(func(i int) error {
				return service.mint(ctx, pairs, i)
			}); err != nil {
				handler.RecordError(ctx)
				logger.Error().Err(err).Msg("Minting error")
				return err
			}

			for range cfg.SwapPerIteration {
				pairsToCall, smartAccountsToCall, err := service.selectPairAndAccount()
				if err != nil {
					handler.RecordError(ctx)
					return err
				}
				if err := parallelizeAcrossN(len(pairsToCall), func(i int) error {
					handler.RecordFromToCall(ctx, int64(smartAccountsToCall[i]-1), int64(pairsToCall[i]-1))
					return service.swap(ctx, pairs, smartAccountsToCall[i]-1, pairsToCall[i]-1)
				}); err != nil {
					handler.RecordError(ctx)
					logger.Error().Err(err).Msg("Swap error")
					return err
				}
			}

			if err := service.parallelizeAcrossShards(func(i int) error {
				return service.burn(ctx, pairs, i)
			}); err != nil {
				handler.RecordError(ctx)
				logger.Error().Err(err).Msg("Burn error")
				return err
			}
			logger.Info().Msg("Iteration finished.")
		}
	}
}
