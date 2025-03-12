package relayer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/relayer/internal/l1"
	"github.com/ethereum/go-ethereum/ethclient"
)

type RelayerConfig struct {
	EventListenerConfig *l1.EventListenerConfig
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig: l1.DefaultEventListenerConfig(),
	}
}

type RelayerService struct {
	srv.Service
}

func New(
	ctx context.Context,
	database db.DB,
	clock common.Timer,
	config *RelayerConfig,
	l1Client *ethclient.Client, // TODO(oclaw) use interface
) (*RelayerService, error) {
	logger := logging.NewLogger("relayer")

	l1Storage, err := l1.NewEventStorage(
		ctx,
		database,
		clock,
		nil, // TODO(oclaw) metrics
		logger,
	)
	if err != nil {
		return nil, err
	}

	l1EventListener, err := l1.NewEventListener(
		l1Client,
		config.EventListenerConfig,
		l1Storage,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return &RelayerService{
		Service: srv.NewService(
			logger,
			l1EventListener,
			// TODO(oclaw) L1 finality ensurer
			// TODO(oclaw) L2 transaction sender
		),
	}, nil
}
