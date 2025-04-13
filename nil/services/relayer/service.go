package relayer

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

type RelayerConfig struct {
	EventListenerConfig     *l1.EventListenerConfig
	FinalityEnsurerConfig   *l1.FinalityEnsurerConfig
	TransactionSenderConfig *l2.TransactionSenderConfig
	L2ContractConfig        *l2.ContractConfig
	TelemetryConfig         *telemetry.Config
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig:     l1.DefaultEventListenerConfig(),
		FinalityEnsurerConfig:   l1.DefaultFinalityEnsurerConfig(),
		TransactionSenderConfig: l2.DefaultTransactionSenderConfig(),
		L2ContractConfig:        l2.DefaultContractConfig(),
		TelemetryConfig: &telemetry.Config{
			ServiceName: "relayer",
		},
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

	if err := telemetry.Init(ctx, config.TelemetryConfig); err != nil {
		return nil, fmt.Errorf("failed to init telemetry: %w", err)
	}

	storageMetrics, err := storage.NewTableMetrics()
	if err != nil {
		return nil, err
	}

	l1Storage, err := l1.NewEventStorage(
		ctx,
		database,
		clock,
		storageMetrics,
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

	eventListenerMetrics, err := l1.NewEventListenerMetrics()
	if err != nil {
		return nil, err
	}

	rs.L1EventListener, err = l1.NewEventListener(
		config.EventListenerConfig,
		clock,
		l1Client,
		l1Contract,
		l1Storage,
		eventListenerMetrics,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	l2Storage := l2.NewEventStorage(
		ctx,
		database,
		clock,
		storageMetrics,
		rs.Logger,
	)

	finalityEnsurerMetrics, err := l1.NewFinalityEnsurerMetrics()
	if err != nil {
		return nil, err
	}

	rs.L1FinalityEnsurer, err = l1.NewFinalityEnsurer(
		config.FinalityEnsurerConfig,
		l1Client,
		clock,
		rs.Logger,
		l1Storage,
		l2Storage,
		finalityEnsurerMetrics,
		rs.L1EventListener,
	)
	if err != nil {
		return nil, err
	}

	l2Client, l2SmartAccountAddr, err := l2.InitL2(ctx, rs.Logger, config.L2ContractConfig)
	if err != nil {
		return nil, err
	}
	if !l2SmartAccountAddr.IsEmpty() && len(config.L2ContractConfig.SmartAccountAddress) == 0 {
		rs.Logger.Info().
			Str("smart_account_address", l2SmartAccountAddr.Hex()).
			Msg("using automatically created smart account address for L2 operations")
		config.L2ContractConfig.SmartAccountAddress = l2SmartAccountAddr.Hex()
	}

	l2Contract, err := l2.NewL2ContractWrapper(
		ctx,
		config.L2ContractConfig,
		l2Client,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	transactionSenderMetrics, err := l2.NewTransactionSenderMetrics()
	if err != nil {
		return nil, err
	}

	rs.L2TransactionSender, err = l2.NewTransactionSender(
		config.TransactionSenderConfig,
		l2Storage,
		rs.Logger,
		clock,
		rs.L1FinalityEnsurer,
		transactionSenderMetrics,
		l2Contract,
	)
	if err != nil {
		return nil, err
	}

	return rs, nil
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
