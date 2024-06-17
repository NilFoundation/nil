package service

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/rs/zerolog"
)

type Service struct {
	client     client.Client
	privateKey *ecdsa.PrivateKey
	logger     zerolog.Logger
}

// NewService initializes a new Service with the given client
func NewService(c client.Client, pk string) *Service {
	s := &Service{
		client: c,
		logger: logging.NewLogger("cliService"),
	}

	if len(pk) != 0 {
		privateKey, err := crypto.HexToECDSA(pk)
		s.logger.Err(err).Msg("Failed to parse private key")
		check.PanicIfErr(err)
		s.privateKey = privateKey
	}

	return s
}
