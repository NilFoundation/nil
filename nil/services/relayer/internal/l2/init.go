package l2

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/ethereum/go-ethereum/crypto"
)

func InitL2(
	ctx context.Context,
	logger logging.Logger,
	config *ContractConfig,
) (client.Client, types.Address, error) {
	l2Client := rpc.NewClient(
		config.Endpoint,
		logger,
	)
	ver, err := l2Client.ClientVersion(ctx)
	if err != nil {
		return nil, types.EmptyAddress, err
	}
	logger.Info().Str("client_version", ver).Msg("connected to L2")

	if !config.DebugMode {
		return l2Client, types.EmptyAddress, nil
	}

	addr, err := initDebugL2SmartAccount(ctx, l2Client, logger, config)
	if err != nil {
		return nil, types.EmptyAddress, fmt.Errorf("failed to init debug L2 smart account: %w", err)
	}

	return l2Client, addr, nil
}

func initDebugL2SmartAccount(
	ctx context.Context,
	l2Client client.Client,
	logger logging.Logger,
	config *ContractConfig,
) (types.Address, error) {
	logger = logger.With().Bool("debug_mode", true).Logger()
	logger.Warn().Msg("key/account generation enabled, this is not recommended for production")

	var keyExists bool
	_, err := os.Stat(config.PrivateKeyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return types.EmptyAddress, err
		}
	} else {
		keyExists = true
		logger.Debug().Str("key_path", config.PrivateKeyPath).Msg("found generated key")
	}

	var key *ecdsa.PrivateKey
	if !keyExists {
		logger.Warn().
			Str("key_path", config.PrivateKeyPath).
			Msg("private key not found, generating it")

		key, err = crypto.GenerateKey()
		if err != nil {
			return types.EmptyAddress, err
		}

		if err := crypto.SaveECDSA(config.PrivateKeyPath, key); err != nil {
			return types.EmptyAddress, err
		}
		logger.Info().Msg("key generated")
	} else {
		key, err = crypto.LoadECDSA(config.PrivateKeyPath)
		if err != nil {
			return types.EmptyAddress, fmt.Errorf("failed to load private key: %w", err)
		}
	}

	faucetClient := faucet.NewClient(config.FaucetAddress)

	debugDeployer := cliservice.NewService(ctx, l2Client, key, faucetClient)

	amount := types.NewValueFromUint64(2_000_000_000_000_000)
	fee := types.NewFeePackFromFeeCredit(types.NewValueFromUint64(200_000_000_000_000))

	salt := types.NewUint256(0)
	if len(config.SmartAccountSalt) > 0 {
		salt, err = types.NewUint256FromDecimal(config.SmartAccountSalt)
		if err != nil {
			return types.EmptyAddress, fmt.Errorf("failed to parse smart account salt: %w", err)
		}
	} else {
		logger.Warn().Msg("smart account salt is not set, using default")
	}

	addr, err := debugDeployer.CreateSmartAccount(
		types.BaseShardId,
		salt,
		amount,
		fee,
		&key.PublicKey,
	)
	if errors.Is(err, cliservice.ErrSmartAccountExists) {
		if config.SmartAccountAddress != "" {
			check.PanicIfNotf(
				config.SmartAccountAddress == addr.Hex(),
				"smart account address mismatch: %s vs %s",
				config.SmartAccountAddress, addr.Hex(),
			)
		}
		logger.Info().Str("smart_account_address", addr.Hex()).Msg("smart account already exists")
		return addr, nil
	}
	if err != nil {
		return types.EmptyAddress, fmt.Errorf("failed to create smart account: %w", err)
	}

	return addr, nil
}
