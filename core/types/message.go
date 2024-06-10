package types

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	ssz "github.com/ferranbt/fastssz"
	"github.com/rs/zerolog/log"
)

type MessageKind int

const (
	InMessageKind MessageKind = iota
	OutMessageKind
)

type MessageIndex uint64

func (mi MessageIndex) Bytes() []byte {
	return ssz.MarshalUint64(nil, uint64(mi))
}

func (mi *MessageIndex) SetBytes(b []byte) {
	*mi = MessageIndex(ssz.UnmarshallUint64(b))
}

func BytesToMessageIndex(b []byte) MessageIndex {
	var mi MessageIndex
	mi.SetBytes(b)
	return mi
}

type Message struct {
	Internal  bool             `json:"internal" ch:"internal"`
	Seqno     uint64           `json:"seqno,omitempty" ch:"seqno"`
	GasPrice  Uint256          `json:"gasPrice,omitempty" ch:"gas_price" ssz-size:"32"`
	GasLimit  Uint256          `json:"gasLimit,omitempty" ch:"gas_limit" ssz-size:"32"`
	From      Address          `json:"from,omitempty" ch:"from"`
	To        Address          `json:"to,omitempty" ch:"to"`
	Value     Uint256          `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data      Code             `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	Signature common.Signature `json:"signature,omitempty" ch:"signature"`
}

type messageDigest struct {
	Internal bool
	Seqno    uint64
	GasPrice Uint256 `ssz-size:"32"`
	GasLimit Uint256 `ssz-size:"32"`
	From     Address
	To       Address
	Value    Uint256 `ssz-size:"32"`
	Data     Code    `ssz-max:"24576"`
}

// interfaces
var (
	_ common.Hashable = new(Message)
	_ ssz.Marshaler   = new(Message)
	_ ssz.Unmarshaler = new(Message)
)

func (m *Message) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	common.FatalIf(err, log.Logger, "Can't get message hash")

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
		Internal: m.Internal,
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
