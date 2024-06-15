package service

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/rs/zerolog"
)

type Service struct {
	client     client.Client
	privateKey *ecdsa.PrivateKey
	logger     zerolog.Logger
}

// NewService initializes a new Service with the given client
func NewService(c client.Client, privateKey *ecdsa.PrivateKey) *Service {
	s := &Service{
		client: c,
		logger: logging.NewLogger("cliService"),
	}

	s.privateKey = privateKey

	return s
}
