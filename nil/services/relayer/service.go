package relayer

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

type RelayerConfig struct {
	EventListenerConfig     *l1.EventListenerConfig
	FinalityEnsurerConfig   *l1.FinalityEnsurerConfig
	TransactionSenderConfig *l2.TransactionSenderConfig
	L2ContractConfig        *l2.ContractConfig
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig:     l1.DefaultEventListenerConfig(),
		FinalityEnsurerConfig:   l1.DefaultFinalityEnsurerConfig(),
		TransactionSenderConfig: l2.DefaultTransactionSenderConfig(),
		L2ContractConfig:        l2.DefaultContractConfig(),
	}
}

type RelayerService struct {
	Logger              logging.Logger
	L1EventListener     *l1.EventListener
	L1FinalityEnsurer   *l1.FinalityEnsurer
	L2TransactionSender *l2.TransactionSender
}

func New(
	ctx context.Context,
	database db.DB,
	clock clockwork.Clock,
	config *RelayerConfig,
	l1Client l1.EthClient,
) (*RelayerService, error) {
	rs := &RelayerService{
		Logger: logging.NewLogger("relayer"),
	}

	l1Storage, err := l1.NewEventStorage(
		ctx,
		database,
		clock,
		nil, // TODO(oclaw) metrics
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	l1Contract, err := l1.NewL1ContractWrapper(
		l1Client,
		config.EventListenerConfig.BridgeMessengerContractAddress,
		config.L2ContractConfig.ContractAddress,
	)
	if err != nil {
		return nil, err
	}

	rs.L1EventListener, err = l1.NewEventListener(
		config.EventListenerConfig,
		clock,
		l1Client,
		l1Contract,
		l1Storage,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	l2Storage := l2.NewEventStorage(
		ctx,
		database,
		clock,
		nil, // TODO(oclaw) metrics
		rs.Logger,
	)

	rs.L1FinalityEnsurer, err = l1.NewFinalityEnsurer(
		config.FinalityEnsurerConfig,
		l1Client,
		clock,
		rs.Logger,
		l1Storage,
		l2Storage,
		rs.L1EventListener,
	)
	if err != nil {
		return nil, err
	}

	l2Client, err := rs.initL2(ctx, config.L2ContractConfig)
	if err != nil {
		return nil, err
	}

	l2Contract, err := l2.NewL2ContractWrapper(
		ctx,
		config.L2ContractConfig,
		l2Client,
	)
	if err != nil {
		return nil, err
	}

	rs.L2TransactionSender, err = l2.NewTransactionSender(
		config.TransactionSenderConfig,
		l2Storage,
		rs.Logger,
		clock,
		rs.L1FinalityEnsurer,
		l2Contract,
	)
	if err != nil {
		return nil, err
	}

	return rs, nil
}

func (rs *RelayerService) initL2(ctx context.Context, config *l2.ContractConfig) (client.Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("failed to init L2: invalid config: %w", err)
	}

	l2Client := rpc.NewClient(
		config.Endpoint,
		rs.Logger,
	)
	ver, err := l2Client.ClientVersion(ctx)
	if err != nil {
		return nil, err
	}
	rs.Logger.Info().Str("client_version", ver).Msg("connected to L2")

	var keyExists bool
	_, err = os.Stat(config.PrivateKeyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		keyExists = true
		rs.Logger.Debug().Str("key_path", config.PrivateKeyPath).Msg("found generated key")
	}

	if !keyExists {
		rs.Logger.Info().
			Str("key_path", config.PrivateKeyPath).
			Msg("private key not found, generating it")

		if key, err := crypto.GenerateKey(); err != nil {
			return nil, err
		} else {
			if err := crypto.SaveECDSA(config.PrivateKeyPath, key); err != nil {
				return nil, err
			}
		}
		rs.Logger.Info().Msg("key generated")
	}

	// TODO deploy relayer's L2 smart account (if it is not exist)

	return l2Client, nil
}

func (rs *RelayerService) Run(ctx context.Context) error {
	eg, gCtx := errgroup.WithContext(ctx)

	eventListenerStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L1EventListener.Run(gCtx, eventListenerStarted)
	})

	finalityEnsurerStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L1FinalityEnsurer.Run(gCtx, finalityEnsurerStarted)
	})

	transactionSenderStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L2TransactionSender.Run(ctx, transactionSenderStarted)
	})

	return eg.Wait()
}
