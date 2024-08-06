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
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

const (
	IncrementContractCode = `0x6080604052348015600e575f80fd5b506101498061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80636d4ce63c14610038578063d09de08a14610056575b5f80fd5b610040610060565b60405161004d919061009a565b60405180910390f35b61005e610068565b005b5f8054905090565b60015f8082825461007991906100e0565b92505081905550565b5f819050919050565b61009481610082565b82525050565b5f6020820190506100ad5f83018461008b565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6100ea82610082565b91506100f583610082565b925082820190508082111561010d5761010c6100b3565b5b9291505056fea264697066735822122017563c6eae8ec11268139f122ea07e70216c9c1c8827f4360d0170f2187491a464736f6c63430008190033`
	IncrementCalldata     = `0xd09de08a`
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
		txHashCaller, addr, err := client.DeployContract(shardId, wallets[0], types.BuildDeployPayload(hexutil.FromHex(IncrementContractCode), common.EmptyHash), types.Value{}, privateKeys[0])
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
					hash, err = client.SendMessageViaWallet(wallet, hexutil.FromHex(IncrementCalldata),
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
