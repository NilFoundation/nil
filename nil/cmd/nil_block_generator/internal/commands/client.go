package commands

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	cliservice_common "github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/cmd/nild/nildconfig"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
)

func backgroundNilNode(cfg *nildconfig.Config) {
	database, err := db.NewBadgerDb(cfg.DB.Path)
	if err != nil {
		fmt.Printf("failed to create new BadgerDB\n")
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer func() {
		database.Close()
		stop()
	}()
	exitCode := nilservice.Run(ctx, cfg.Config, database, nil,
		func(ctx context.Context) error {
			return database.LogGC(ctx, cfg.DB.DiscardRatio, cfg.DB.GcFrequency)
		})
	if exitCode != 0 {
		fmt.Printf("nilservice failed with code %d\n", exitCode)
	}
}

func waitStartNil(rpcEndpoint string) error {
	client := rpc.NewClient(rpcEndpoint, zerolog.Nop())
	ctx := context.Background()
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.DelayExponential(100*time.Millisecond, time.Second),
		},
		zerolog.Nop(),
	)

	err := retryRunner.Do(ctx, func(context.Context) error {
		_, err := client.GetBlock(ctx, types.ShardId(1), transport.BlockNumber(0), false)
		return err
	})
	return err
}

func RunNilNode(rpcEndpoint string) error {
	cfg := &nildconfig.Config{
		Config: nilservice.NewDefaultConfig(),
		DB:     db.NewDefaultBadgerDBOptions(),
		ReadThrough: &nildconfig.ReadThroughOptions{
			ForkMainAtBlock: transport.LatestBlockNumber,
		},
	}
	cfg.HttpUrl = rpcEndpoint
	cfg.NShards = 2
	go backgroundNilNode(cfg)
	return waitStartNil(rpcEndpoint) // make sure if service started
}

func GetRpcClient(rpcEndpoint string, logger zerolog.Logger) *rpc.Client {
	return rpc.NewClientWithDefaultHeaders(
		rpcEndpoint,
		logger,
		map[string]string{
			"User-Agent": "nil-block-generatr-cli/" + version.GetGitRevCount(),
		},
	)
}

func GetFaucetRpcClient(faucetEndpoint string) *faucet.Client {
	return faucet.NewClient(faucetEndpoint)
}

func CreateCliService(rpcEndpoint, hexKey string, logger zerolog.Logger) (*cliservice.Service, error) {
	faucet := GetFaucetRpcClient(rpcEndpoint)
	rpc := GetRpcClient(rpcEndpoint, logger)
	service := cliservice.NewService(context.Background(), rpc, nil, faucet)
	err := service.GenerateKeyFromHex(hexKey)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func CreateNewSmartAccount(rpcEndpoint string, logger zerolog.Logger) (string, string, error) {
	keygen := cliservice.NewService(context.Background(), &rpc.Client{}, nil, nil)
	if err := keygen.GenerateNewKey(); err != nil {
		return "", "", err
	}
	hexKey := keygen.GetPrivateKey()

	salt := types.NewUint256(0)
	amount := types.NewValueFromUint64(2_000_000_000_000_000)
	fee := types.NewFeePackFromFeeCredit(types.NewValueFromUint64(200_000_000_000_000))

	srv, err := CreateCliService(rpcEndpoint, hexKey, logger)
	if err != nil {
		return "", "", err
	}
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return "", "", err
	}
	smartAccount, err := srv.CreateSmartAccount(types.BaseShardId, salt, amount, fee, &privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	return smartAccount.Hex(), hexKey, nil
}

func DeployContract(rpcEndpoint, smartAccountAdr, path, hexKey string, args []string, logger zerolog.Logger) (string, error) {
	binPath := path + ".bin"
	abiPath := path + ".abi"
	bytecode, err := cliservice_common.ReadBytecode(binPath, abiPath, args)
	if err != nil {
		return "", err
	}

	salt := types.NewUint256(0)
	payload := types.BuildDeployPayload(bytecode, common.Hash(salt.Bytes32()))

	amount := types.Value0

	srv, err := CreateCliService(rpcEndpoint, hexKey, logger)
	if err != nil {
		return "", err
	}

	var smartAccountAddress types.Address
	if err := smartAccountAddress.Set(smartAccountAdr); err != nil {
		return "", fmt.Errorf("invalid smartAccount address: %w", err)
	}

	txnHash, addr, err := srv.DeployContractViaSmartAccount(types.BaseShardId, smartAccountAddress, payload, amount)
	if err != nil {
		return "", err
	}

	_, err = srv.WaitForReceipt(txnHash)
	if err != nil {
		return "", err
	}

	return addr.Hex(), nil
}

func CallContract(rpcEndpoint, smartAccountAdr, hexKey string, calls []Call, logger zerolog.Logger) (string, error) {
	srv, err := CreateCliService(rpcEndpoint, hexKey, logger)
	if err != nil {
		return "", err
	}

	var smartAccountAddress types.Address
	if err := smartAccountAddress.Set(smartAccountAdr); err != nil {
		return "", fmt.Errorf("invalid smartAccount address: %w", err)
	}

	tokensStr := make([]string, 0)
	tokens, err := cliservice_common.ParseTokens(tokensStr)
	if err != nil {
		return "", err
	}

	amount := types.Value0
	fee := types.NewFeePackFromGas(100_000)

	ctx := context.Background()
	client := GetRpcClient(rpcEndpoint, logger)
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return "", err
	}

	callParams := make([]rpc.CallParam, len(calls))
	for i, call := range calls {
		var address types.Address
		if err := address.Set(call.Address); err != nil {
			return "", fmt.Errorf("invalid contract address: %w", err)
		}

		abi, err := cliservice_common.ReadAbiFromFile(call.AbiPath)
		if err != nil {
			return "", err
		}

		calldata, err := cliservice_common.PrepareArgs(abi, call.Method, call.Args)
		if err != nil {
			return "", err
		}

		callParams[i].Bytecode = calldata
		callParams[i].Address = address
		callParams[i].Count = call.Count
	}
	transactionHash, err := rpc.RunContractBatch(ctx, client, smartAccountAddress, callParams, fee, amount, tokens, privateKey)
	if err != nil {
		return "", err
	}

	receipt, err := srv.WaitForReceipt(transactionHash)
	if err != nil {
		return "", err
	}
	if receipt.Success &&
		len(receipt.OutReceipts) >= 1 &&
		!receipt.Flags.GetBit(types.TransactionFlagInternal) {
		internalReceipt := receipt.OutReceipts[0]
		if internalReceipt.Success &&
			internalReceipt.Flags.GetBit(types.TransactionFlagInternal) {
			return internalReceipt.BlockHash.Hex(), nil
		}
	}
	fmt.Println(receipt)
	return "", fmt.Errorf("invalid receipt for %s", transactionHash.Hex())
}
