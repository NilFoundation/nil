package types

import (
	"crypto/ecdsa"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/crypto"
)

type MessageKind int

const (
	InMessageKind MessageKind = iota
	OutMessageKind
)

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
	Internal bool    `json:"internal" ch:"internal"`
	Deploy   bool    `json:"deploy,omitempty" ch:"deploy"`
	ChainId  ChainId `json:"chainId" ch:"chainId"`
	Seqno    Seqno   `json:"seqno,omitempty" ch:"seqno"`
	GasPrice Uint256 `json:"gasPrice,omitempty" ch:"gas_price" ssz-size:"32"`
	GasLimit Uint256 `json:"gasLimit,omitempty" ch:"gas_limit" ssz-size:"32"`
	From     Address `json:"from,omitempty" ch:"from"`
	To       Address `json:"to,omitempty" ch:"to"`
	Value    Uint256 `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data     Code    `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	// This field should always be at the end of the structure for easy signing
	Signature Signature `json:"signature,omitempty" ch:"signature" ssz-max:"256"`
}

type ExternalMessage struct {
	Deploy   bool      `json:"deploy,omitempty" ch:"deploy"`
	To       Address   `json:"to,omitempty" ch:"to"`
	ChainId  ChainId   `json:"chainId" ch:"chainId"`
	Seqno    Seqno     `json:"seqno,omitempty" ch:"seqno"`
	Data     Code      `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	AuthData Signature `json:"authData,omitempty" ch:"auth_data" ssz-max:"256"`
}

type InternalMessagePayload struct {
	Deploy   bool    `json:"deploy,omitempty" ch:"deploy"`
	GasLimit Uint256 `json:"gasLimit,omitempty" ch:"gas_limit" ssz-size:"32"`
	To       Address `json:"to,omitempty" ch:"to"`
	Value    Uint256 `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data     Code    `json:"data,omitempty" ch:"data" ssz-max:"24576"`
}

type messageDigest struct {
	Deploy  bool
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
		Deploy:   m.Deploy,
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
		Deploy:   m.Deploy,
		To:       m.To,
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
		Deploy:  m.Deploy,
		Seqno:   m.Seqno,
		To:      m.To,
		Data:    m.Data,
		ChainId: m.ChainId,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m *Message) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Deploy:  m.Deploy,
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
