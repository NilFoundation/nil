package service

import (
	"github.com/NilFoundation/nil/core/crypto"
)

// GenerateNewKey generates a new private key
func (s *Service) GenerateNewKey() error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	s.privateKey = privateKey
	return nil
}

// GenerateKeyFromHex generates a private key from a hexadecimal string
func (s *Service) GenerateKeyFromHex(hexKey string) error {
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return err
	}

	s.privateKey = privateKey
	return nil
}

// GetPrivateKey returns the private key in hexadecimal format
func (s *Service) GetPrivateKey() string {
	privHex := crypto.PrivateKeyToEthereumFormat(s.privateKey)

	return privHex
}
