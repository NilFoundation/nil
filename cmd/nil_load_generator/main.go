package main

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/cli/service"
	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

const (
	IncrementContractCode = `0x6080604052348015600e575f80fd5b506101498061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80636d4ce63c14610038578063d09de08a14610056575b5f80fd5b610040610060565b60405161004d919061009a565b60405180910390f35b61005e610068565b005b5f8054905090565b60015f8082825461007991906100e0565b92505081905550565b5f819050919050565b61009481610082565b82525050565b5f6020820190506100ad5f83018461008b565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6100ea82610082565b91506100f583610082565b925082820190508082111561010d5761010c6100b3565b5b9291505056fea264697066735822122017563c6eae8ec11268139f122ea07e70216c9c1c8827f4360d0170f2187491a464736f6c63430008190033`
	IncrementCalldata     = `0xd09de08a`
)

func main() {
	logger := logging.NewLogger("nil_load_generator")

	rootCmd := &cobra.Command{
		Use:   "nil_loadgen",
		Short: "Run nil load generator",
	}

	rpcEndpoint := rootCmd.Flags().String("port", "http://127.0.0.1:8529/", "rpc endpoint")
	contractCallDelay := rootCmd.Flags().Duration("delay", 500*time.Millisecond, "delay between contracts call")
	mainKeysPath := rootCmd.Flags().String("keys", "keys.yaml", "path to yaml file with main keys")

	check.PanicIfErr(rootCmd.Execute())

	err := execution.LoadMainKeys(*mainKeysPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load main keys")
	}

	client := rpc_client.NewClient(*rpcEndpoint)
	service := service.NewService(client, execution.MainPrivateKey)

	shardIdList, err := client.GetShardIdList()
	nShards := len(shardIdList)
	if err != nil {
		logger.Fatal().Err(err).Msg("Can't get shards number")
	}

	var addresses []types.Address
	for i := 1; i < nShards+1; i++ {
		txHashCaller, addr, err := client.DeployContract(types.ShardId(i), types.MainWalletAddress, hexutil.FromHex(IncrementContractCode), nil, execution.MainPrivateKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("Error during deploy contract")
		}
		_, err = service.WaitForReceipt(types.MainWalletAddress.ShardId(), txHashCaller)
		if err != nil {
			logger.Fatal().Err(err).Msg("Can't get receipt for contract")
		}
		addresses = append(addresses, addr)
	}

	for {
		addrNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(addresses))))
		if err != nil {
			logger.Error().Err(err).Msg("Error during get random contract address")
		}

		_, err = client.SendMessageViaWallet(types.MainWalletAddress, hexutil.FromHex(IncrementCalldata), types.NewUint256(0), addresses[addrNumber.Int64()], execution.MainPrivateKey)
		if err != nil {
			logger.Error().Err(err).Msg("Error during contract call")
		}
		time.Sleep(*contractCallDelay)
	}
}
