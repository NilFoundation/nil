package types

import (
	"crypto/ecdsa"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

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

type MessageFlags struct {
	BitFlags[uint8]
}

func (flags MessageFlags) Value() (driver.Value, error) {
	return flags.Bits, nil
}

var _ driver.Value = new(MessageFlags)

type ChainId uint64

const DefaultChainId = ChainId(0)

const (
	MessageFlagInternal int = iota
	MessageFlagDeploy
	MessageFlagRefund
	MessageFlagBounce
)

type Message struct {
	Flags   MessageFlags `json:"flags" ch:"flags"`
	ChainId ChainId      `json:"chainId" ch:"chainId"`
	Seqno   Seqno        `json:"seqno,omitempty" ch:"seqno"`
	// TODO: This field is not used right now, but it should be used in the future
	GasPrice Value             `json:"gasPrice,omitempty" ch:"gas_price" ssz-size:"32"`
	GasLimit Gas               `json:"gasLimit,omitempty" ch:"gas_limit"`
	From     Address           `json:"from,omitempty" ch:"from"`
	To       Address           `json:"to,omitempty" ch:"to"`
	RefundTo Address           `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo Address           `json:"bounceTo,omitempty" ch:"bounceTo"`
	Value    Value             `json:"value,omitempty" ch:"value" ssz-size:"32"`
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
	Kind     MessageKind       `json:"kind,omitempty" ch:"kind"`
	Bounce   bool              `json:"bounce,omitempty" ch:"bounce"`
	GasLimit Gas               `json:"gasLimit,omitempty" ch:"gas_limit"`
	To       Address           `json:"to,omitempty" ch:"to"`
	RefundTo Address           `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo Address           `json:"bounceTo,omitempty" ch:"bounceTo"`
	Currency []CurrencyBalance `json:"currency,omitempty" ch:"currency" ssz-max:"256"`
	Value    Value             `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data     Code              `json:"data,omitempty" ch:"data" ssz-max:"24576"`
}

type messageDigest struct {
	Flags   MessageFlags
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
	_ error           = new(MessageError)
)

func NewEmptyMessage() *Message {
	return &Message{
		GasPrice:  NewValueFromUint64(0),
		Value:     NewValueFromUint64(0),
		Currency:  make([]CurrencyBalance, 0),
		Signature: make(Signature, 0),
	}
}

func (m *Message) Hash() common.Hash {
	if m.IsExternal() {
		return m.toExternal().Hash()
	}
	h, err := common.PoseidonSSZ(m)
	check.PanicIfErr(err)
	return h
}

func (m *Message) Sign(key *ecdsa.PrivateKey) error {
	ext := m.toExternal()
	if err := ext.Sign(key); err != nil {
		return err
	}
	m.Signature = ext.AuthData
	return nil
}

func (m *Message) toExternal() *ExternalMessage {
	if m.IsInternal() {
		panic("cannot convert internal message to external message")
	}
	var kind MessageKind
	switch {
	case m.IsDeploy():
		kind = DeployMessageKind
	case m.IsRefund():
		kind = RefundMessageKind
	default:
		kind = ExecutionMessageKind
	}
	return &ExternalMessage{
		Kind:     kind,
		To:       m.To,
		ChainId:  m.ChainId,
		Seqno:    m.Seqno,
		Data:     m.Data,
		AuthData: m.Signature,
	}
}

func (m *Message) VerifyFlags() error {
	if m.IsInternal() {
		num := 0
		if m.IsDeploy() {
			num++
		}
		if m.IsRefund() {
			num++
		}
		if m.IsBounce() {
			num++
		}
		if num > 1 {
			return errors.New("internal message cannot be deploy, refund or bounce at the same time")
		}
	} else if m.IsRefund() || m.IsBounce() {
		return errors.New("external message cannot be bounce or refund")
	}
	return nil
}

func (m *Message) IsInternal() bool {
	return m.Flags.GetBit(MessageFlagInternal)
}

func (m *Message) IsExternal() bool {
	return !m.IsInternal()
}

func (m *Message) IsExecution() bool {
	return !m.Flags.GetBit(MessageFlagDeploy) && !m.Flags.GetBit(MessageFlagRefund)
}

func (m *Message) IsBounce() bool {
	return m.Flags.GetBit(MessageFlagBounce)
}

func (m *Message) IsDeploy() bool {
	return m.Flags.GetBit(MessageFlagDeploy)
}

func (m *Message) IsRefund() bool {
	return m.Flags.GetBit(MessageFlagRefund)
}

func (m *InternalMessagePayload) ToMessage(from Address, seqno Seqno) *Message {
	msg := &Message{
		Flags:    MessageFlagsFromKind(true, m.Kind),
		To:       m.To,
		RefundTo: m.RefundTo,
		BounceTo: m.BounceTo,
		From:     from,
		Value:    m.Value,
		Currency: m.Currency,
		Data:     m.Data,
		GasLimit: m.GasLimit,
		Seqno:    seqno,
	}
	if m.Bounce {
		msg.Flags.SetBit(MessageFlagBounce)
	}

	return msg
}

func (m *ExternalMessage) Hash() common.Hash {
	h, err := common.PoseidonSSZ(m)
	check.PanicIfErr(err)
	return h
}

func (m *ExternalMessage) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Flags:   MessageFlagsFromKind(false, m.Kind),
		Seqno:   m.Seqno,
		To:      m.To,
		Data:    m.Data,
		ChainId: m.ChainId,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m *Message) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Flags:   m.Flags,
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

func NewMessageFlags(flags ...int) MessageFlags {
	return MessageFlags{NewBitFlags[uint8](flags...)}
}

func MessageFlagsFromKind(internal bool, kind MessageKind) MessageFlags {
	flags := make([]int, 0, 2)
	if internal {
		flags = append(flags, MessageFlagInternal)
	}
	switch kind {
	case DeployMessageKind:
		flags = append(flags, MessageFlagDeploy)
	case RefundMessageKind:
		flags = append(flags, MessageFlagRefund)
	case ExecutionMessageKind: // do nothing
	}
	return NewMessageFlags(flags...)
}

func (m MessageFlags) String() string {
	var res string
	if m.GetBit(MessageFlagInternal) {
		res += "Internal"
	} else {
		res += "External"
	}
	if m.GetBit(MessageFlagDeploy) {
		res += ", Deploy"
	}
	if m.GetBit(MessageFlagRefund) {
		res += ", Refund"
	}
	if m.GetBit(MessageFlagBounce) {
		res += ", Bounce"
	}
	return res
}

func (m MessageFlags) MarshalJSON() ([]byte, error) {
	var res string
	if m.GetBit(MessageFlagInternal) {
		res += "\"Internal\""
	} else {
		res += "\"External\""
	}
	if m.GetBit(MessageFlagDeploy) {
		res += ", \"Deploy\""
	}
	if m.GetBit(MessageFlagRefund) {
		res += ", \"Refund\""
	}
	if m.GetBit(MessageFlagBounce) {
		res += ", \"Bounce\""
	}
	return []byte(fmt.Sprintf("[%s]", res)), nil
}

func (m *MessageFlags) UnmarshalJSON(data []byte) error {
	m.Clear()
	var s []string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, v := range s {
		switch v {
		case "Internal":
			m.SetBit(MessageFlagInternal)
		case "Deploy":
			m.SetBit(MessageFlagDeploy)
		case "Refund":
			m.SetBit(MessageFlagRefund)
		case "Bounce":
			m.SetBit(MessageFlagBounce)
		}
	}
	return nil
}

const (
	MessageStatusSuccess MessageStatus = iota
	// All codes below are considered as errors
	MessageStatusOther
	MessageStatusExecution
	MessageStatusOutOfGas
	MessageStatusSeqno
	MessageStatusBounce
	MessageStatusBuyGas
	MessageStatusValidation
	MessageStatusInsufficientBalance
	MessageStatusNoAccount
	MessageStatusCodeStoreOutOfGas
	MessageStatusDepth
	MessageStatusContractAddressCollision
	MessageStatusExecutionReverted
	MessageStatusMaxCodeSizeExceeded
	MessageStatusMaxInitCodeSizeExceeded
	MessageStatusInvalidJump
	MessageStatusWriteProtection
	MessageStatusReturnDataOutOfBounds
	MessageStatusGasUintOverflow
	MessageStatusInvalidCode
	MessageStatusNonceUintOverflow
	MessageStatusInvalidInputLength
	MessageStatusCrossShardMessage
	MessageStatusStopToken
)

type MessageStatus uint32

type MessageError struct {
	Status MessageStatus
	Inner  error
}

func NewMessageError(status MessageStatus, err error) *MessageError {
	check.PanicIfNot(err != nil)
	return &MessageError{status, err}
}

func (m *MessageError) Error() string {
	return m.Inner.Error()
}

func (m *MessageError) Unwrap() error {
	return m.Inner
}
