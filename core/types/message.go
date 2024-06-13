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

type messageDigest struct {
	Internal bool
	Deploy   bool
	ChainId  ChainId
	Seqno    Seqno
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
	check.PanicIfErr(err)
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
		ChainId:  m.ChainId,
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

	m.Signature = Signature(sig)

	return nil
}
