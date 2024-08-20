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
	IncrementContractCode = `0x608060405234801561000f575f80fd5b506103808061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c806357b8a50f146100435780636d4ce63c1461005f578063796d7f561461007d575b5f80fd5b61005d6004803603810190610058919061014b565b6100ad565b005b6100676100ed565b6040516100749190610185565b60405180910390f35b61009760048036038101906100929190610232565b610101565b6040516100a491906102a9565b60405180910390f35b805f808282829054906101000a900460030b6100c991906102ef565b92506101000a81548163ffffffff021916908360030b63ffffffff16021790555050565b5f805f9054906101000a900460030b905090565b5f600190509392505050565b5f80fd5b5f80fd5b5f8160030b9050919050565b61012a81610115565b8114610134575f80fd5b50565b5f8135905061014581610121565b92915050565b5f602082840312156101605761015f61010d565b5b5f61016d84828501610137565b91505092915050565b61017f81610115565b82525050565b5f6020820190506101985f830184610176565b92915050565b5f819050919050565b6101b08161019e565b81146101ba575f80fd5b50565b5f813590506101cb816101a7565b92915050565b5f80fd5b5f80fd5b5f80fd5b5f8083601f8401126101f2576101f16101d1565b5b8235905067ffffffffffffffff81111561020f5761020e6101d5565b5b60208301915083600182028301111561022b5761022a6101d9565b5b9250929050565b5f805f604084860312156102495761024861010d565b5b5f610256868287016101bd565b935050602084013567ffffffffffffffff81111561027757610276610111565b5b610283868287016101dd565b92509250509250925092565b5f8115159050919050565b6102a38161028f565b82525050565b5f6020820190506102bc5f83018461029a565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6102f982610115565b915061030483610115565b925082820190507fffffffffffffffffffffffffffffffffffffffffffffffffffffffff800000008112637fffffff82131715610344576103436102c2565b5b9291505056fea26469706673582212205d80a4424f46b63fd21864ea4f86d4e8c43cf3351e590d82c7c556c2664ebe1564736f6c63430008150033`
	IncrementCalldata     = `0x57b8a50f000000000000000000000000000000000000000000000000000000000000000b`
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
