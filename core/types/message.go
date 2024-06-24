package types

import (
	"crypto/ecdsa"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/ethereum/go-ethereum/crypto"
)

type MessageKind uint8

// TODO: Maybe separated this enum for internal/external case
const (
	ExecutionMessageKind MessageKind = iota
	DeployMessageKind
	RefundMessageKind
)

func (k MessageKind) String() string {
	switch k {
	case ExecutionMessageKind:
		return "ExecutionMessageKind"
	case DeployMessageKind:
		return "DeployMessageKind"
	case RefundMessageKind:
		return "RefundMessageKind"
	}
	panic("unknown MessageKind")
}

type Seqno uint64

func (seqno Seqno) Uint64() uint64 {
	return uint64(seqno)
}

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

type ChainId uint64

const DefaultChainId = ChainId(0)

type Message struct {
	Internal bool        `json:"internal" ch:"internal"`
	Kind     MessageKind `json:"kind,omitempty" ch:"kind"`
	ChainId  ChainId     `json:"chainId" ch:"chainId"`
	Seqno    Seqno       `json:"seqno,omitempty" ch:"seqno"`
	// TODO: This field is not used right now, but it should be used in the future
	GasPrice Uint256           `json:"gasPrice,omitempty" ch:"gas_price" ssz-size:"32"`
	GasLimit Uint256           `json:"gasLimit,omitempty" ch:"gas_limit" ssz-size:"32"`
	From     Address           `json:"from,omitempty" ch:"from"`
	To       Address           `json:"to,omitempty" ch:"to"`
	RefundTo Address           `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo Address           `json:"bounceTo,omitempty" ch:"bounceTo"`
	Value    Uint256           `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Currency []CurrencyBalance `json:"currency,omitempty" ch:"currency" ssz-max:"256"`
	Data     Code              `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	// This field should always be at the end of the structure for easy signing
	Signature Signature `json:"signature,omitempty" ch:"signature" ssz-max:"256"`
}

type ExternalMessage struct {
	Kind     MessageKind `json:"kind,omitempty" ch:"kind"`
	To       Address     `json:"to,omitempty" ch:"to"`
	ChainId  ChainId     `json:"chainId" ch:"chainId"`
	Seqno    Seqno       `json:"seqno,omitempty" ch:"seqno"`
	Data     Code        `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	AuthData Signature   `json:"authData,omitempty" ch:"auth_data" ssz-max:"256"`
}

type InternalMessagePayload struct {
	Kind     MessageKind `json:"kind,omitempty" ch:"kind"`
	GasLimit Uint256     `json:"gasLimit,omitempty" ch:"gas_limit" ssz-size:"32"`
	To       Address     `json:"to,omitempty" ch:"to"`
	RefundTo Address     `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo Address     `json:"bounceTo,omitempty" ch:"bounceTo"`
	Value    Uint256     `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data     Code        `json:"data,omitempty" ch:"data" ssz-max:"24576"`
}

type messageDigest struct {
	Kind    MessageKind
	To      Address
	ChainId ChainId
	Seqno   Seqno
	Data    Code `ssz-max:"24576"`
}

// interfaces
var (
	_ common.Hashable = new(Message)
	_ common.Hashable = new(ExternalMessage)
	_ ssz.Marshaler   = new(Message)
	_ ssz.Unmarshaler = new(Message)
)

func (m *Message) Hash() common.Hash {
	if !m.Internal {
		return m.toExternal().Hash()
	}
	h, err := common.PoseidonSSZ(m)
	check.PanicIfErr(err)
	return h
}

func (m *Message) toExternal() *ExternalMessage {
	if m.Internal {
		panic("cannot convert internal message to external message")
	}
	return &ExternalMessage{
		Kind:     m.Kind,
		To:       m.To,
		ChainId:  m.ChainId,
		Seqno:    m.Seqno,
		Data:     m.Data,
		AuthData: m.Signature,
	}
}

func (m *InternalMessagePayload) ToMessage(from Address, seqno Seqno) *Message {
	return &Message{
		Internal: true,
		Kind:     m.Kind,
		To:       m.To,
		RefundTo: m.RefundTo,
		BounceTo: m.BounceTo,
		From:     from,
		Value:    m.Value,
		Data:     m.Data,
		GasLimit: m.GasLimit,
		Seqno:    seqno,
	}
}

func (m *ExternalMessage) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	check.PanicIfErr(err)
	return h
}

func (m *ExternalMessage) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Kind:    m.Kind,
		Seqno:   m.Seqno,
		To:      m.To,
		Data:    m.Data,
		ChainId: m.ChainId,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m *Message) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Kind:    m.Kind,
		Seqno:   m.Seqno,
		To:      m.To,
		Data:    m.Data,
		ChainId: m.ChainId,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m *ExternalMessage) Sign(key *ecdsa.PrivateKey) error {
	hash, err := m.SigningHash()
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(hash.Bytes(), key)
	if err != nil {
		return err
	}

	m.AuthData = Signature(sig)

	return nil
}
