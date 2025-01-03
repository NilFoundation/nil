package cliservice

import (
	"context"
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/rs/zerolog"
)

type Service struct {
	// ctx is common for all application so we don't need to pass it to each function separately
	ctx          context.Context
	client       client.Client
	privateKey   *ecdsa.PrivateKey
	logger       zerolog.Logger
	faucetClient *faucet.Client
}

// NewService initializes a new Service with the given client
func NewService(ctx context.Context, c client.Client, privateKey *ecdsa.PrivateKey, fc *faucet.Client) *Service {
	s := &Service{
		ctx:          ctx,
		client:       c,
		faucetClient: fc,
		logger:       logging.NewLogger("cliservice"),
	}

	s.privateKey = privateKey

	return s
}
