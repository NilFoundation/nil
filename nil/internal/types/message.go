package types

import (
	"crypto/ecdsa"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type MessageKind uint8

// TODO: Maybe separated this enum for internal/external case
const (
	ExecutionMessageKind MessageKind = iota
	DeployMessageKind
	RefundMessageKind
	ResponseMessageKind
)

func (k MessageKind) String() string {
	switch k {
	case ExecutionMessageKind:
		return "ExecutionMessageKind"
	case DeployMessageKind:
		return "DeployMessageKind"
	case RefundMessageKind:
		return "RefundMessageKind"
	case ResponseMessageKind:
		return "ResponseMessageKind"
	}
	panic("unknown MessageKind")
}

func (k *MessageKind) Set(input string) error {
	switch input {
	case "execution", "ExecutionMessageKind":
		*k = ExecutionMessageKind
	case "deploy", "DeployMessageKind":
		*k = DeployMessageKind
	case "refund", "RefundMessageKind":
		*k = RefundMessageKind
	case "response", "ResponseMessageKind":
		*k = ResponseMessageKind
	default:
		return fmt.Errorf("unknown MessageKind: %s", input)
	}
	return nil
}

func (k MessageKind) Type() string {
	return "MessageKind"
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
	MessageFlagResponse
)

type ForwardKind uint64

const (
	ForwardKindRemaining = iota
	ForwardKindPercentage
	ForwardKindValue
	ForwardKindNone
)

func (k ForwardKind) String() string {
	switch k {
	case ForwardKindRemaining:
		return "ForwardKindRemaining"
	case ForwardKindPercentage:
		return "ForwardKindPercentage"
	case ForwardKindValue:
		return "ForwardKindValue"
	case ForwardKindNone:
		return "ForwardKindNone"
	}
	panic("unknown ForwardKind")
}

func (k *ForwardKind) Set(input string) error {
	switch input {
	case "remaining", "ForwardKindRemaining":
		*k = ForwardKindRemaining
	case "percentage", "ForwardKindPercentage":
		*k = ForwardKindPercentage
	case "value", "ForwardKindValue":
		*k = ForwardKindValue
	case "none", "ForwardKindNone":
		*k = ForwardKindNone
	default:
		return fmt.Errorf("unknown ForwardKind: %s", input)
	}
	return nil
}

func (k ForwardKind) Type() string {
	return "ForwardKind"
}

type Message struct {
	Flags     MessageFlags      `json:"flags" ch:"flags"`
	ChainId   ChainId           `json:"chainId" ch:"chainId"`
	Seqno     Seqno             `json:"seqno,omitempty" ch:"seqno"`
	FeeCredit Value             `json:"feeCredit,omitempty" ch:"fee_credit" ssz-size:"32"`
	From      Address           `json:"from,omitempty" ch:"from"`
	To        Address           `json:"to,omitempty" ch:"to"`
	RefundTo  Address           `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo  Address           `json:"bounceTo,omitempty" ch:"bounceTo"`
	Value     Value             `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Currency  []CurrencyBalance `json:"currency,omitempty" ch:"currency" ssz-max:"256"`
	Data      Code              `json:"data,omitempty" ch:"data" ssz-max:"24576"`

	// These fields are needed for async requests
	RequestId    uint64              `json:"requestId,omitempty" ch:"request_id"`
	RequestChain []*AsyncRequestInfo `json:"response,omitempty" ch:"response" ssz-max:"4096"`

	// This field should always be at the end of the structure for easy signing
	Signature Signature `json:"signature,omitempty" ch:"signature" ssz-max:"256"`
}

type OutboundMessage struct {
	*Message
	ForwardKind ForwardKind `json:"forwardFee,omitempty" ch:"forward_kind"`
}

type ExternalMessage struct {
	Kind      MessageKind `json:"kind,omitempty" ch:"kind"`
	FeeCredit Value       `json:"feeCredit,omitempty" ch:"fee_credit" ssz-size:"32"`
	To        Address     `json:"to,omitempty" ch:"to"`
	ChainId   ChainId     `json:"chainId" ch:"chainId"`
	Seqno     Seqno       `json:"seqno,omitempty" ch:"seqno"`
	Data      Code        `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	AuthData  Signature   `json:"authData,omitempty" ch:"auth_data" ssz-max:"256"`
}

type InternalMessagePayload struct {
	Kind           MessageKind       `json:"kind,omitempty" ch:"kind"`
	Bounce         bool              `json:"bounce,omitempty" ch:"bounce"`
	FeeCredit      Value             `json:"feeCredit,omitempty" ch:"fee_credit" ssz-size:"32"`
	ForwardKind    ForwardKind       `json:"forwardKind,omitempty" ch:"forward_kind"`
	To             Address           `json:"to,omitempty" ch:"to"`
	RefundTo       Address           `json:"refundTo,omitempty" ch:"refundTo"`
	BounceTo       Address           `json:"bounceTo,omitempty" ch:"bounceTo"`
	Currency       []CurrencyBalance `json:"currency,omitempty" ch:"currency" ssz-max:"256"`
	Value          Value             `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data           Code              `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	RequestId      uint64            `json:"requestId,omitempty" ch:"request_id"`
	RequestContext Code              `json:"context,omitempty" ch:"context" ssz-max:"24576"`
}

type messageDigest struct {
	Flags     MessageFlags
	FeeCredit Value `ssz-size:"32"`
	To        Address
	ChainId   ChainId
	Seqno     Seqno
	Data      Code `ssz-max:"24576"`
}

// EvmState contains EVM data to be saved/restored during await request.
type EvmState struct {
	Memory []byte `ssz-max:"10000000"`
	Stack  []byte `ssz-max:"32768"`
	Pc     uint64
}

// AsyncRequestInfo contains information about the incomplete request, that is a request which waits for response to a
// nested request.
type AsyncRequestInfo struct {
	Id     uint64  `json:"id"`
	Caller Address `json:"caller"`
}

// AsyncResponsePayload contains data returned in the response.
type AsyncResponsePayload struct {
	Success    bool
	ReturnData []byte `ssz-max:"10000000"`
}

// AsyncContext contains context of the request. For await requests it contains VM state, which will be restored upon
// the response. For callback requests it contains captured variables(not implemented yet).
type AsyncContext struct {
	IsAwait               bool
	Data                  []byte `ssz-max:"10000000"`
	ResponseProcessingGas Gas
}

// interfaces
var (
	_ common.Hashable = new(Message)
	_ common.Hashable = new(ExternalMessage)
	_ ssz.Marshaler   = new(Message)
	_ ssz.Unmarshaler = new(Message)
)

func NewEmptyMessage() *Message {
	return &Message{
		Value:        NewValueFromUint64(0),
		FeeCredit:    NewValueFromUint64(0),
		Currency:     make([]CurrencyBalance, 0),
		Signature:    make(Signature, 0),
		RequestChain: make([]*AsyncRequestInfo, 0),
	}
}

func (m *Message) Hash() common.Hash {
	if m.IsExternal() {
		return m.toExternal().Hash()
	}
	return common.MustPoseidonSSZ(m)
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
		Kind:      kind,
		FeeCredit: m.FeeCredit,
		To:        m.To,
		ChainId:   m.ChainId,
		Seqno:     m.Seqno,
		Data:      m.Data,
		AuthData:  m.Signature,
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
		if m.IsRequestOrResponse() {
			num++
		}
		if num > 1 {
			return errors.New("internal message cannot be deploy, refund, bounce or async at the same time")
		}
	} else if m.IsRefund() || m.IsBounce() || m.IsRequestOrResponse() {
		return errors.New("external message cannot be bounce, refund or async")
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
	return !m.Flags.IsDeploy() && !m.Flags.IsRefund()
}

func (m *Message) IsBounce() bool {
	return m.Flags.IsBounce()
}

func (m *Message) IsDeploy() bool {
	return m.Flags.IsDeploy()
}

func (m *Message) IsRefund() bool {
	return m.Flags.IsRefund()
}

func (m *Message) IsResponse() bool {
	return m.Flags.IsResponse()
}

func (m *Message) IsRequest() bool {
	return m.IsRequestOrResponse() && !m.IsResponse()
}

func (m *Message) IsRequestOrResponse() bool {
	return m.RequestId != 0
}

func (m *InternalMessagePayload) ToMessage(from Address, seqno Seqno) *Message {
	msg := &Message{
		Flags:     MessageFlagsFromKind(true, m.Kind),
		To:        m.To,
		RefundTo:  m.RefundTo,
		BounceTo:  m.BounceTo,
		From:      from,
		Value:     m.Value,
		Currency:  m.Currency,
		Data:      m.Data,
		FeeCredit: m.FeeCredit,
		RequestId: m.RequestId,
		Seqno:     seqno,
	}
	if m.Bounce {
		msg.Flags.SetBit(MessageFlagBounce)
	}

	return msg
}

func (m *ExternalMessage) Hash() common.Hash {
	return common.MustPoseidonSSZ(m)
}

func (m *ExternalMessage) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Flags:     MessageFlagsFromKind(false, m.Kind),
		FeeCredit: m.FeeCredit,
		Seqno:     m.Seqno,
		To:        m.To,
		Data:      m.Data,
		ChainId:   m.ChainId,
	}

	return common.PoseidonSSZ(&messageDigest)
}

func (m ExternalMessage) ToMessage() *Message {
	return &Message{
		Flags:     MessageFlagsFromKind(false, m.Kind),
		To:        m.To,
		From:      m.To,
		ChainId:   m.ChainId,
		Seqno:     m.Seqno,
		Data:      m.Data,
		Signature: m.AuthData,
		FeeCredit: m.FeeCredit,
	}
}

func (m *Message) SigningHash() (common.Hash, error) {
	messageDigest := messageDigest{
		Flags:     m.Flags,
		FeeCredit: m.FeeCredit,
		Seqno:     m.Seqno,
		To:        m.To,
		Data:      m.Data,
		ChainId:   m.ChainId,
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
	case ResponseMessageKind:
		flags = append(flags, MessageFlagResponse)
	case ExecutionMessageKind: // do nothing
	}
	return NewMessageFlags(flags...)
}

func (m MessageFlags) String() string {
	var res string
	if m.IsInternal() {
		res += "Internal"
	} else {
		res += "External"
	}
	if m.IsDeploy() {
		res += ", Deploy"
	}
	if m.IsRefund() {
		res += ", Refund"
	}
	if m.IsBounce() {
		res += ", Bounce"
	}
	if m.IsResponse() {
		res += ", Response"
	}
	return res
}

func (m MessageFlags) MarshalJSON() ([]byte, error) {
	var res string
	if m.IsInternal() {
		res += "\"Internal\""
	} else {
		res += "\"External\""
	}
	if m.IsDeploy() {
		res += ", \"Deploy\""
	}
	if m.IsRefund() {
		res += ", \"Refund\""
	}
	if m.IsBounce() {
		res += ", \"Bounce\""
	}
	if m.IsResponse() {
		res += ", \"Response\""
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
		case "Response":
			m.SetBit(MessageFlagResponse)
		}
	}
	return nil
}

func (m MessageFlags) IsInternal() bool {
	return m.GetBit(MessageFlagInternal)
}

func (m MessageFlags) IsDeploy() bool {
	return m.GetBit(MessageFlagDeploy)
}

func (m MessageFlags) IsRefund() bool {
	return m.GetBit(MessageFlagRefund)
}

func (m MessageFlags) IsBounce() bool {
	return m.GetBit(MessageFlagBounce)
}

func (m MessageFlags) IsResponse() bool {
	return m.GetBit(MessageFlagResponse)
}
