package nil_load_generator

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os/signal"
	"strconv"
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
	mintCurrency0Amount = 10000
	mintCurrency1Amount = 10000
	swapAmount          = 1000
)

var wallets []uniswap.Wallet

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

func initializeWalletsAndServices(shardIdList []types.ShardId, client *rpc_client.Client, service *cliservice.Service, faucet *faucet.Client) ([]uniswap.Wallet, []*cliservice.Service, error) {
	res := make([]uniswap.Wallet, len(shardIdList))
	services := make([]*cliservice.Service, len(shardIdList))

	for i, shardId := range shardIdList {
		var err error
		res[i], err = uniswap.NewWallet(service, shardId)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize wallet for shard %s: %w", shardId, err)
		}

		services[i] = cliservice.NewService(client, res[i].PrivateKey, faucet)
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
	client := rpc_client.NewClient(cfg.Endpoint, logger)
	logging.SetupGlobalLogger(cfg.LogLevel)

	service := cliservice.NewService(client, execution.MainPrivateKey, faucet)
	shardIdList, err := client.GetShardIdList()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get shard id list")
		return err
	}
	logger.Info().Msg("Creating wallets...")
	var services []*cliservice.Service
	wallets, services, err = initializeWalletsAndServices(shardIdList, client, service, faucet)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize wallets and services")
		return err
	}

	logger.Info().Msg("Wallets created successfully.")
	contractNames := []string{"UniswapV2Factory", "Currency", "UniswapV2Pair", "UniswapV2Router01"}
	contracts, err := compileContracts(contractNames)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to compile contracts")
		return err
	}

	currencies := make([]*uniswap.Currency, len(shardIdList)*2)
	factories := make([]*uniswap.Factory, len(shardIdList))
	pairs := make([]*uniswap.Pair, len(shardIdList))

	if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
		logger.Info().Msgf("Deploying currencies on shard %v", shardIdList[i])

		for j := range 2 {
			currency := uniswap.NewCurrency(contracts["Currency"], "currency"+strconv.Itoa(i*2+j), wallets[i])
			currencies[i*2+j] = currency
			if err := currency.Deploy(services[i], wallets[i]); err != nil {
				return fmt.Errorf("failed to deploy currency on shard %v: %w", shardIdList[i], err)
			}
		}

		logger.Info().Msgf("Deploying factory on shard %v", shardIdList[i])
		factories[i] = uniswap.NewFactory(contracts["UniswapV2Factory"])
		if err := factories[i].Deploy(services[i], wallets[i], wallets[i].Addr); err != nil {
			return fmt.Errorf("failed to deploy factory on shard %v: %w", shardIdList[i], err)
		}

		logger.Info().Msgf("Creating pair on shard %v", shardIdList[i])
		if err := factories[i].CreatePair(services[i], client, wallets[i], currencies[i*2].Addr, currencies[i*2+1].Addr); err != nil {
			return fmt.Errorf("failed to create pair on shard %v: %w", shardIdList[i], err)
		}

		logger.Info().Msgf("Initializing pair on shard %v", shardIdList[i])
		pairAddress, err := factories[i].GetPair(services[i], currencies[i*2].Addr, currencies[i*2+1].Addr)
		if err != nil {
			return fmt.Errorf("failed to get pair on shard %v: %w", shardIdList[i], err)
		}

		pairs[i] = uniswap.NewPair(contracts["UniswapV2Pair"], pairAddress)
		if err := pairs[i].Initialize(services[i], client, wallets[i], currencies[i*2], currencies[i*2+1]); err != nil {
			return fmt.Errorf("failed to initialize pair on shard %v: %w", shardIdList[i], err)
		}

		return nil
	}); err != nil {
		logger.Error().Err(err).Msg("Deployment and initialization error")
		return err
	}

	logger.Info().Msg("Starting main loop.")
	checkBalanceCounterDownInt := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if checkBalanceCounterDownInt == 0 {
				checkBalanceCounterDownInt = int(cfg.CheckBalance)
				logger.Info().Msg("Checking balance and minting currencies.")
				err := uniswap.TopUpBalance(client, services, wallets, currencies)
				if err != nil {
					return err
				}
			}
			checkBalanceCounterDownInt--
			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				logger.Info().Msgf("Minting liqudity for wallet %s on shard %v", wallets[i].Addr, shardIdList[i])
				return pairs[i].Mint(
					services[i], client, wallets[i], wallets[i].Addr,
					[]types.CurrencyBalance{
						{Currency: currencies[i*2].Id, Balance: types.NewValueFromUint64(mintCurrency0Amount)},
						{Currency: currencies[i*2+1].Id, Balance: types.NewValueFromUint64(mintCurrency1Amount)},
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
				walletsToCall, err := randomPermutation(shardIdList, numberCalls.Uint64())
				if err != nil {
					return err
				}
				if err := parallelizeAcrossN(int(numberCalls.Int64()), func(i int) error {
					whoWantSwap := walletsToCall[i] - 1
					whatPairHeWant := pairsToCall[i] - 1
					reserve0, reserve1, err := pairs[whatPairHeWant].GetReserves(services[whoWantSwap])
					if err != nil {
						return err
					}
					expectedOutputAmount := calculateOutputAmount(big.NewInt(swapAmount), reserve0, reserve1)
					logger.Info().Msgf("User: %v, Pair: %v, AmountSend: %d,  AmountGet: %d, CurrencyFrom: %s, CurrencyTo %s", whoWantSwap, whatPairHeWant, swapAmount, expectedOutputAmount, currencies[whatPairHeWant*2].Id, currencies[whatPairHeWant*2+1].Id)

					if err = pairs[whatPairHeWant].Swap(services[whoWantSwap], client, wallets[whoWantSwap], wallets[whoWantSwap].Addr, big.NewInt(0), expectedOutputAmount, types.NewValueFromUint64(swapAmount), currencies[whatPairHeWant*2].Id); err != nil {
						return err
					}
					return nil
				}); err != nil {
					logger.Error().Err(err).Msg("Minting error")
					return err
				}
			}

			if err := parallelizeAcrossN(len(shardIdList), func(i int) error {
				logger.Info().Msgf("Burn liquidity for user wallet %s on shard %v", wallets[i].Addr, shardIdList[i])
				userLpBalance, err := pairs[i].GetCurrencyBalanceOf(services[i], wallets[i].Addr)
				if err != nil {
					return err
				}
				if userLpBalance.Uint64() > 0 {
					return pairs[i].Burn(
						services[i], client, wallets[i], wallets[i].Addr,
						types.CurrencyId(pairs[i].Addr),
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
