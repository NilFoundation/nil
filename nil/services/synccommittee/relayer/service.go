package relayer

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/relayer/internal/l2"
)

type RelayerService struct {
	srv.Service
}

func New() *RelayerService {
	// TODO(oclaw)
	// - init storages
	// - init L1/L2 clients

	logger := logging.NewLogger("relayer")

	return &RelayerService{
		Service: srv.NewService(
			logger,
			&l1.EventListener{},     // TODO(oclaw) call ctor
			&l1.FinalityEnsurer{},   // TODO(oclaw) call ctor
			&l2.TransactionSender{}, // TODO(oclaw) call ctor
		),
	}
}
