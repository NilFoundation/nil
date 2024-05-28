package types

import (
	"crypto/ecdsa"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	ssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type MessageKind int

const (
	InMessageKind MessageKind = iota
	OutMessageKind
)

type Message struct {
	Seqno     uint64           `json:"seqno,omitempty"`
	GasPrice  Uint256          `json:"gasPrice,omitempty"`
	GasLimit  Uint256          `json:"gasLimit,omitempty"`
	From      common.Address   `json:"from,omitempty"`
	To        common.Address   `json:"to,omitempty"`
	Value     Uint256          `json:"value,omitempty"`
	Data      Code             `json:"data,omitempty" ssz-max:"24576"`
	Signature common.Signature `json:"signature,omitempty"`
}

type messageDigest struct {
	Seqno    uint64
	GasPrice Uint256
	GasLimit Uint256
	From     common.Address
	To       common.Address
	Value    Uint256
	Data     Code `ssz-max:"24576"`
}

// interfaces
var (
	_ common.Hashable = new(Message)
	_ ssz.Marshaler   = new(Message)
	_ ssz.Unmarshaler = new(Message)
)

func (m *Message) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't get message hash")
	}
	return h
}

func (m *Message) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Seqno:    m.Seqno,
		GasPrice: m.GasPrice,
		GasLimit: m.GasLimit,
		From:     m.From,
		To:       m.To,
		Value:    m.Value,
		Data:     m.Data,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m *Message) Sign(key *ecdsa.PrivateKey) error {
	hash, err := m.SigningHash()
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(hash.Bytes(), key)
	if err != nil {
		return err
	}

	m.Signature = common.Signature(sig)

	return nil
}

func (m *Message) ValidateSignature(pubBytes []byte) (bool, error) {
	if len(m.Signature) != 65 {
		return false, nil
	}

	hash, err := m.SigningHash()
	if err != nil {
		return false, err
	}

	return crypto.VerifySignature(pubBytes, hash.Bytes(), m.Signature[:64]), nil
}
