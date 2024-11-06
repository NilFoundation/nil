package cliservice

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/rs/zerolog"
)

type Service struct {
	client       client.Client
	privateKey   *ecdsa.PrivateKey
	logger       zerolog.Logger
	faucetClient *faucet.Client
}

// NewService initializes a new Service with the given client
func NewService(c client.Client, privateKey *ecdsa.PrivateKey, fc *faucet.Client) *Service {
	s := &Service{
		client:       c,
		faucetClient: fc,
		logger:       logging.NewLogger("cliservice"),
	}

	s.privateKey = privateKey

	return s
}
