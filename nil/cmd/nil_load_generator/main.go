package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/cmd/nil_load_generator/metrics"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

func RandomPermutation(n, amount uint64) ([]uint64, error) {
	arr := make([]uint64, n)
	for i := range n {
		arr[i] = i
	}

	for i := n - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return nil, err
		}
		j := jBig.Uint64()
		arr[i], arr[j] = arr[j], arr[i]
	}

	return arr[:amount], nil
}

func TopUpBalance(service *cliservice.Service, wallets []types.Address, mh *metrics.MetricsHandler) error {
	const topUpAmount = uint64(500_000_000)
	for _, wallet := range wallets {
		balance, err := service.GetBalance(wallet)
		if err != nil {
			return err
		}
		if balance.Uint64() < topUpAmount {
			if err := service.TopUpViaFaucet(wallet, types.NewValueFromUint64(topUpAmount)); err != nil {
				return err
			}
			if err := HandleWalletBalanceMetrics(service, mh, wallet, topUpAmount); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetValueUsed(receipt *jsonrpc.RPCReceipt) types.Value {
	res := receipt.GasUsed.ToValue(receipt.GasPrice)
	for _, outReceipt := range receipt.OutReceipts {
		res.Add(GetValueUsed(outReceipt))
	}
	return res
}

func HandleWalletBalanceMetrics(service *cliservice.Service, mh *metrics.MetricsHandler, wallet types.Address, approxAmount uint64) error {
	balance, err := service.GetBalance(wallet)
	if err != nil {
		return err
	}

	// Update the wallet balance in the metrics handler
	mh.SetCurrentWalletBalance(context.Background(), balance.Uint64(), wallet)
	mh.SetCurrentApproxWalletBalance(context.Background(), approxAmount, wallet)

	return nil
}

func createWallets(service *cliservice.Service, shardIds []types.ShardId, mh *metrics.MetricsHandler) ([]types.Address, []*ecdsa.PrivateKey, error) {
	privateKeys := make([]*ecdsa.PrivateKey, 0)
	wallets := make([]types.Address, 0)
	for _, shardId := range shardIds {
		ownerPrivateKey, err := crypto.GenerateKey()
		if err != nil {
			return nil, nil, err
		}
		walletAddr, err := service.CreateWallet(shardId, types.NewUint256(0), types.NewValueFromUint64(1_000_000_000), &ownerPrivateKey.PublicKey)
		if err != nil {
			walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&ownerPrivateKey.PublicKey))
			walletAddr = service.ContractAddress(shardId, *types.NewUint256(0), walletCode)
		}
		balance, err := service.GetBalance(walletAddr)
		if err != nil {
			return nil, nil, err
		}
		if err := HandleWalletBalanceMetrics(service, mh, walletAddr, balance.Uint64()); err != nil {
			return nil, nil, err
		}
		wallets = append(wallets, walletAddr)
		privateKeys = append(privateKeys, ownerPrivateKey)
	}
	return wallets, privateKeys, nil
}

func deployContracts(client *rpc_client.Client, wallets []types.Address, privateKeys []*ecdsa.PrivateKey, incrementContractCode []byte, mh *metrics.MetricsHandler) ([]types.Address, error) {
	contractsCall := make([]types.Address, 0)
	for i, wallet := range wallets {
		service := cliservice.NewService(client, privateKeys[i])

		txHashCaller, addr, err := service.DeployContractViaWallet(wallet.ShardId(), wallet, types.BuildDeployPayload(incrementContractCode, wallet.Hash()), types.Value{})
		if err != nil {
			return nil, err
		}
		receipt, err := service.WaitForReceipt(wallet.ShardId(), txHashCaller)
		if err != nil {
			return nil, err
		}

		valueUsed := GetValueUsed(receipt)
		if err := HandleWalletBalanceMetrics(service, mh, wallet, -valueUsed.Uint64()); err != nil {
			return nil, err
		}
		contractsCall = append(contractsCall, addr)
	}
	return contractsCall, nil
}

func main() {
	componentName := "nil_load_generator"
	logger := logging.NewLogger(componentName)

	incrementContractCode, err := contracts.GetCode("tests/Counter")
	if err != nil {
		logger.Err(err).Send()
		panic("Counter code is not found")
	}

	abi, err := contracts.GetAbi("tests/Counter")
	if err != nil {
		logger.Err(err).Send()
		panic("Counter ABI is not found")
	}

	incrementCalldata, err := abi.Pack("add", int32(11))
	if err != nil {
		logger.Err(err).Send()
		panic("Failed to create counter calldata")
	}

	rootCmd := &cobra.Command{
		Use:   componentName,
		Short: "Run nil load generator",
	}

	rpcEndpoint := rootCmd.Flags().String("endpoint", "http://127.0.0.1:8529/", "rpc endpoint")
	contractCallDelay := rootCmd.Flags().Duration("delay", 500*time.Millisecond, "delay between contracts call")
	checkBalanceFrequency := rootCmd.Flags().Uint32("check-balance", 10, "frequency of balance check in iterations")
	exportMetrics := rootCmd.Flags().Bool("metrics", false, "export metrics via grpc")

	check.PanicIfErr(rootCmd.Execute())

	if err := telemetry.Init(context.Background(), &telemetry.Config{ServiceName: componentName, ExportMetrics: *exportMetrics}); err != nil {
		logger.Err(err).Send()
		panic("Can't init telemetry")
	}
	defer telemetry.Shutdown(context.Background())

	mh, err := metrics.NewMetricsHandler(componentName)
	if err != nil {
		logger.Err(err).Send()
		panic("Can't init metrics handler")
	}

	client := rpc_client.NewClient(*rpcEndpoint, logger)
	service := cliservice.NewService(client, execution.MainPrivateKey)

	shardIdList, err := client.GetShardIdList()
	if err != nil {
		mh.RecordError(context.Background())
		logger.Err(err).Send()
		panic("Can't get shards number")
	}
	nShards := len(shardIdList)

	wallets, privateKeys, err := createWallets(service, shardIdList, mh)
	if err != nil {
		logger.Err(err).Send()
		panic("Can't create wallets")
	}

	contractsCall, err := deployContracts(client, wallets, privateKeys, incrementContractCode, mh)
	if err != nil {
		mh.RecordError(context.Background())
		logger.Err(err).Send()
		panic("Failed to deploy contracts")
	}

	checkBalanceCounterDownInt := int(*checkBalanceFrequency)
	for {
		var wg sync.WaitGroup
		for i, wallet := range wallets {
			numberCalls, err := rand.Int(rand.Reader, big.NewInt(int64(len(contractsCall))))
			if err != nil {
				mh.RecordError(context.Background())
				logger.Error().Err(err).Msg("Error during get random calls number")
			}
			addrToCall, err := RandomPermutation(uint64(nShards-1), numberCalls.Uint64())
			if err != nil {
				mh.RecordError(context.Background())
				logger.Error().Err(err).Msg("Error during get random contract address")
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				var hash common.Hash
				for _, addr := range addrToCall {
					hash, err = client.SendMessageViaWallet(wallet, incrementCalldata,
						types.GasToValue(200_000), types.Value{}, []types.CurrencyBalance{},
						contractsCall[addr],
						privateKeys[i])
					if err != nil {
						mh.RecordError(context.Background())
						logger.Error().Err(err).Msg("Error during contract call")
					}
					receipt, err := service.WaitForReceipt(wallet.ShardId(), hash)
					if err != nil {
						mh.RecordError(context.Background())
						logger.Error().Err(err).Msg("Can't get receipt for contract")
					}
					valueUsed := GetValueUsed(receipt)
					if err := HandleWalletBalanceMetrics(service, mh, wallet, -valueUsed.Uint64()); err != nil {
						mh.RecordError(context.Background())
						logger.Error().Err(err).Msg("Can't get balance")
					}
					mh.RecordFromToCall(context.Background(), int64(i+1), int64(addr+1))
				}
			}()
		}
		wg.Wait()

		checkBalanceCounterDownInt--
		if checkBalanceCounterDownInt == 0 {
			if err := TopUpBalance(service, wallets, mh); err != nil {
				mh.RecordError(context.Background())
				logger.Error().Err(err).Msg("Error during top up balance")
			}
			checkBalanceCounterDownInt = int(*checkBalanceFrequency)
		}
		logger.Info().Msg("Iteration done")
		time.Sleep(*contractCallDelay)
	}
}
