package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
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

func main() {
	logger := logging.NewLogger("nil_load_generator")

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
		Use:   "nil_loadgen",
		Short: "Run nil load generator",
	}

	rpcEndpoint := rootCmd.Flags().String("endpoint", "http://127.0.0.1:8529/", "rpc endpoint")
	contractCallDelay := rootCmd.Flags().Duration("delay", 500*time.Millisecond, "delay between contracts call")

	check.PanicIfErr(rootCmd.Execute())

	client := rpc_client.NewClient(*rpcEndpoint, logger)
	service := cliservice.NewService(client, execution.MainPrivateKey)

	shardIdList, err := client.GetShardIdList()
	nShards := len(shardIdList)
	if err != nil {
		logger.Fatal().Err(err).Msg("Can't get shards number")
	}
	privateKeys := make([]*ecdsa.PrivateKey, 0)
	wallets := make([]types.Address, 0)
	contractsCall := make([]types.Address, 0)
	for _, shardId := range shardIdList {
		ownerPrivateKey, err := crypto.GenerateKey()
		if err != nil {
			logger.Fatal().Err(err).Msg("Can't generate private key")
		}
		walletAddr, err := service.CreateWallet(shardId, types.NewUint256(0), types.NewValueFromUint64(1_000_000_000), &ownerPrivateKey.PublicKey)
		if err != nil {
			logger.Error().Err(err).Msg("Can't create wallet")
			walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&ownerPrivateKey.PublicKey))
			walletAddr = service.ContractAddress(shardId, *types.NewUint256(0), walletCode)
			logger.Info().Msg("Using already created wallet")
		}
		wallets = append(wallets, walletAddr)
		privateKeys = append(privateKeys, ownerPrivateKey)
	}

	for _, shardId := range shardIdList {
		txHashCaller, addr, err := client.DeployContract(shardId, wallets[0], types.BuildDeployPayload(incrementContractCode, common.EmptyHash), types.Value{}, privateKeys[0])
		if err != nil {
			logger.Error().Err(err).Msg("Error during deploy contract, maybe contract already deployed")
		}
		_, err = service.WaitForReceipt(wallets[0].ShardId(), txHashCaller)
		if err != nil {
			logger.Error().Err(err).Msg("Can't get receipt for contract. Maybe duplicate contract deploy")
		}
		contractsCall = append(contractsCall, addr)
	}

	for {
		var wg sync.WaitGroup
		for i, wallet := range wallets {
			numberCalls, err := rand.Int(rand.Reader, big.NewInt(int64(len(contractsCall))))
			if err != nil {
				logger.Error().Err(err).Msg("Error during get random calls number")
			}
			addrToCall, err := RandomPermutation(uint64(nShards-1), numberCalls.Uint64())
			if err != nil {
				logger.Error().Err(err).Msg("Error during get random contract address")
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				var hash common.Hash
				for _, addr := range addrToCall {
					hash, err = client.SendMessageViaWallet(wallet, incrementCalldata,
						types.Gas(100_000).ToValue(types.DefaultGasPrice), types.Value{}, []types.CurrencyBalance{},
						contractsCall[addr],
						privateKeys[i])
					if err != nil {
						logger.Error().Err(err).Msg("Error during contract call")
					}
					_, err = service.WaitForReceipt(wallet.ShardId(), hash)
					if err != nil {
						logger.Error().Err(err).Msg("Can't get receipt for contract")
					}
				}
			}()
		}
		wg.Wait()
		logger.Info().Msg("Iteration done")
		time.Sleep(*contractCallDelay)
	}
}
