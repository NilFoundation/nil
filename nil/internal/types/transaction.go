package types

import (
	"crypto/ecdsa"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/ethereum/go-ethereum/crypto"
)

type TransactionKind uint8

// TODO: Maybe separated this enum for internal/external case
const (
	ExecutionTransactionKind TransactionKind = iota
	DeployTransactionKind
	RefundTransactionKind
	ResponseTransactionKind
)

func (k TransactionKind) String() string {
	switch k {
	case ExecutionTransactionKind:
		return "ExecutionTransactionKind"
	case DeployTransactionKind:
		return "DeployTransactionKind"
	case RefundTransactionKind:
		return "RefundTransactionKind"
	case ResponseTransactionKind:
		return "ResponseTransactionKind"
	}
	panic("unknown TransactionKind")
}

func (k *TransactionKind) Set(input string) error {
	switch input {
	case "execution", "ExecutionTransactionKind":
		*k = ExecutionTransactionKind
	case "deploy", "DeployTransactionKind":
		*k = DeployTransactionKind
	case "refund", "RefundTransactionKind":
		*k = RefundTransactionKind
	case "response", "ResponseTransactionKind":
		*k = ResponseTransactionKind
	default:
		return fmt.Errorf("unknown TransactionKind: %s", input)
	}
	return nil
}

func (k TransactionKind) Type() string {
	return "TransactionKind"
}

type Seqno uint64

func (seqno Seqno) Uint64() uint64 {
	return uint64(seqno)
}

func (seqno Seqno) String() string {
	return strconv.FormatUint(uint64(seqno), 10)
}

type TransactionIndex uint64

const TransactionIndexSize = 8

func (ti TransactionIndex) Bytes() []byte {
	return ssz.MarshalUint64(nil, uint64(ti))
}

func (ti *TransactionIndex) SetBytes(b []byte) {
	*ti = TransactionIndex(ssz.UnmarshallUint64(b))
}

func (ti TransactionIndex) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalUint64(nil, uint64(ti)), nil
}

func (ti *TransactionIndex) MarshalSSZTo(buf []byte) ([]byte, error) {
	return ssz.MarshalUint64(buf, uint64(*ti)), nil
}

func (ti *TransactionIndex) SizeSSZ() int {
	return TransactionIndexSize
}

func (ti *TransactionIndex) UnmarshalSSZ(b []byte) error {
	ti.SetBytes(b)
	return nil
}

func (ti *TransactionIndex) UnmarshalNil(buf []byte) error {
	return ti.UnmarshalSSZ(buf)
}

func (ti TransactionIndex) MarshalNil() ([]byte, error) {
	return ti.MarshalSSZ()
}

func BytesToTransactionIndex(b []byte) TransactionIndex {
	var ti TransactionIndex
	ti.SetBytes(b)
	return ti
}

type TransactionFlags struct {
	BitFlags[uint8]
}

func NewTransactionFlagsFromBits(bits uint8) TransactionFlags {
	return TransactionFlags{BitFlags: BitFlags[uint8]{Bits: bits}}
}

func (m TransactionFlags) Value() (driver.Value, error) {
	return m.Bits, nil
}

var _ driver.Value = new(TransactionFlags)

type ChainId uint64

const DefaultChainId = ChainId(0)

const (
	TransactionFlagInternal int = iota
	TransactionFlagDeploy
	TransactionFlagRefund
	TransactionFlagBounce
	TransactionFlagResponse
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

type TransactionDigest struct {
	Flags TransactionFlags `json:"flags" ch:"flags"`
	FeePack
	To      Address `json:"to,omitempty" ch:"to"`
	ChainId ChainId `json:"chainId" ch:"chainId"`
	Seqno   Seqno   `json:"seqno,omitempty" ch:"seqno"`
	Data    Code    `json:"data,omitempty" ch:"data" ssz-max:"24576"`
}

func (d TransactionDigest) MarshalNil() ([]byte, error) {
	return d.MarshalSSZ()
}

type Transaction struct {
	TransactionDigest
	From     Address          `json:"from,omitempty" ch:"from"`
	TxId     TransactionIndex `json:"txId,omitempty" ch:"tx_id"`
	RefundTo Address          `json:"refundTo,omitempty" ch:"refund_to"`
	BounceTo Address          `json:"bounceTo,omitempty" ch:"bounce_to"`
	Value    Value            `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Token    []TokenBalance   `json:"token,omitempty" ch:"token" ssz-max:"256"`

	// These fields are needed for async requests
	RequestId    uint64              `json:"requestId,omitempty" ch:"request_id"`
	RequestChain []*AsyncRequestInfo `json:"response,omitempty" ch:"response" ssz-max:"4096"`

	// This field should always be at the end of the structure for easy signing
	Signature Signature `json:"signature,omitempty" ch:"signature" ssz-max:"256"`
}

type OutboundTransaction struct {
	*Transaction
	TxnHash     common.Hash `json:"txnHash" ch:"txn_hash"`
	ForwardKind ForwardKind `json:"forwardKind,omitempty" ch:"forward_kind"`
}

type ExternalTransaction struct {
	Kind TransactionKind `json:"kind,omitempty" ch:"kind"`
	FeePack
	To       Address   `json:"to,omitempty" ch:"to"`
	ChainId  ChainId   `json:"chainId" ch:"chainId"`
	Seqno    Seqno     `json:"seqno,omitempty" ch:"seqno"`
	Data     Code      `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	AuthData Signature `json:"authData,omitempty" ch:"auth_data" ssz-max:"256"`
}

func (tx *ExternalTransaction) UnmarshalNil(buf []byte) error {
	return tx.UnmarshalSSZ(buf)
}

func (tx ExternalTransaction) MarshalNil() ([]byte, error) {
	return tx.MarshalSSZ()
}

type InternalTransactionPayload struct {
	Kind        TransactionKind `json:"kind,omitempty" ch:"kind"`
	Bounce      bool            `json:"bounce,omitempty" ch:"bounce"`
	FeeCredit   Value           `json:"feeCredit,omitempty" ch:"fee_credit" ssz-size:"32"`
	ForwardKind ForwardKind     `json:"forwardKind,omitempty" ch:"forward_kind"`
	To          Address         `json:"to,omitempty" ch:"to"`
	RefundTo    Address         `json:"refundTo,omitempty" ch:"refund_to"`
	BounceTo    Address         `json:"bounceTo,omitempty" ch:"bounce_to"`
	Token       []TokenBalance  `json:"token,omitempty" ch:"token" ssz-max:"256"`
	Value       Value           `json:"value,omitempty" ch:"value" ssz-size:"32"`
	Data        Code            `json:"data,omitempty" ch:"data" ssz-max:"24576"`
	RequestId   uint64          `json:"requestId,omitempty" ch:"request_id"`
}

func (p *InternalTransactionPayload) UnmarshalNil(buf []byte) error {
	return p.UnmarshalSSZ(buf)
}

func (p InternalTransactionPayload) MarshalNil() ([]byte, error) {
	return p.MarshalSSZ()
}

type FeePack struct {
	FeeCredit            Value `json:"feeCredit,omitempty" ch:"fee_credit" ssz-size:"32"`
	MaxPriorityFeePerGas Value `json:"maxPriorityFeePerGas,omitempty" ch:"max_priority_fee_per_gas" ssz-size:"32"`
	MaxFeePerGas         Value `json:"maxFeePerGas,omitempty" ch:"max_fee_per_gas" ssz-size:"32"`
}

func NewFeePack() FeePack {
	return FeePack{
		FeeCredit:            NewZeroValue(),
		MaxPriorityFeePerGas: NewZeroValue(),
		MaxFeePerGas:         NewZeroValue(),
	}
}

func NewFeePackFromGas(gas Gas) FeePack {
	return FeePack{
		FeeCredit:            GasToValue(uint64(gas)),
		MaxPriorityFeePerGas: NewZeroValue(),
		MaxFeePerGas:         MaxFeePerGasDefault,
	}
}

func NewFeePackFromFeeCredit(feeCredit Value) FeePack {
	return FeePack{FeeCredit: feeCredit, MaxPriorityFeePerGas: NewZeroValue(), MaxFeePerGas: MaxFeePerGasDefault}
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

func (a AsyncRequestInfo) Value() (driver.Value, error) {
	return []any{a.Id, a.Caller}, nil
}

// AsyncResponsePayload contains data returned in the response.
type AsyncResponsePayload struct {
	Success    bool
	ReturnData []byte `ssz-max:"10000000"`
}

func (p *AsyncResponsePayload) UnmarshalNil(buf []byte) error {
	return p.UnmarshalSSZ(buf)
}

func (p AsyncResponsePayload) MarshalNil() ([]byte, error) {
	return p.MarshalSSZ()
}

// AsyncContext contains the context of the request. For await requests, it contains the VM state, which will be
// restored upon receiving the response. For callback requests, it contains captured variables.
type AsyncContext struct {
	ResponseProcessingGas Gas `json:"gas"`
}

func (c *AsyncContext) UnmarshalNil(buf []byte) error {
	return c.UnmarshalSSZ(buf)
}

func (c AsyncContext) MarshalNil() ([]byte, error) {
	return c.MarshalSSZ()
}

// interfaces
var (
	_ common.Hashable              = new(Transaction)
	_ common.Hashable              = new(ExternalTransaction)
	_ serialization.NilMarshaler   = new(Transaction)
	_ serialization.NilUnmarshaler = new(Transaction)
)

func NewEmptyTransaction() *Transaction {
	return &Transaction{
		TransactionDigest: TransactionDigest{
			FeePack: NewFeePack(),
		},
		Value:        NewZeroValue(),
		Token:        make([]TokenBalance, 0),
		Signature:    make(Signature, 0),
		RequestChain: make([]*AsyncRequestInfo, 0),
	}
}

func (t *Transaction) UnmarshalNil(buf []byte) error {
	return t.UnmarshalSSZ(buf)
}

func (t Transaction) MarshalNil() ([]byte, error) {
	return t.MarshalSSZ()
}

func (m *Transaction) Hash() common.Hash {
	if m.IsExternal() {
		return m.toExternal().Hash()
	}
	return ToShardedHash(common.MustKeccak(m), m.To.ShardId())
}

func (m *Transaction) Sign(key *ecdsa.PrivateKey) error {
	ext := m.toExternal()
	if err := ext.Sign(key); err != nil {
		return err
	}
	m.Signature = ext.AuthData
	return nil
}

func (m *Transaction) toExternal() *ExternalTransaction {
	if m.IsInternal() {
		panic("cannot convert internal transaction to external transaction")
	}
	var kind TransactionKind
	switch {
	case m.IsDeploy():
		kind = DeployTransactionKind
	case m.IsRefund():
		kind = RefundTransactionKind
	default:
		kind = ExecutionTransactionKind
	}
	return &ExternalTransaction{
		Kind:     kind,
		FeePack:  m.FeePack,
		To:       m.To,
		ChainId:  m.ChainId,
		Seqno:    m.Seqno,
		Data:     m.Data,
		AuthData: m.Signature,
	}
}

func (m *Transaction) VerifyFlags() error {
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
			return errors.New("internal transaction cannot be deploy, refund, bounce or async at the same time")
		}
	} else if m.IsRefund() || m.IsBounce() || m.IsRequestOrResponse() {
		return errors.New("external transaction cannot be bounce, refund or async")
	}
	if m.To.ShardId().IsMainShard() && !m.From.ShardId().IsMainShard() {
		return errors.New("transaction to main shard is not allowed from a regular shard")
	}
	return nil
}

func (m *Transaction) IsInternal() bool {
	return m.Flags.GetBit(TransactionFlagInternal)
}

func (m *Transaction) IsExternal() bool {
	return !m.IsInternal()
}

func (m *Transaction) IsExecution() bool {
	return !m.Flags.IsDeploy() && !m.Flags.IsRefund()
}

func (m *Transaction) IsBounce() bool {
	return m.Flags.IsBounce()
}

func (m *Transaction) IsDeploy() bool {
	return m.Flags.IsDeploy()
}

func (m *Transaction) IsRefund() bool {
	return m.Flags.IsRefund()
}

func (m *Transaction) IsResponse() bool {
	return m.Flags.IsResponse()
}

func (m *Transaction) IsRequest() bool {
	return m.IsRequestOrResponse() && !m.IsResponse()
}

func (m *Transaction) IsRequestOrResponse() bool {
	return m.RequestId != 0
}

func (m *Transaction) IsSystem() bool {
	return m.To.ShardId().IsMainShard()
}

func (m *Transaction) TransactionGasPrice(baseFeePerGas Value) (Value, error) {
	gasPrice := baseFeePerGas.Add(m.MaxPriorityFeePerGas)
	// Zero MaxFeePerGas means no limit
	if !m.MaxFeePerGas.IsZero() && gasPrice.Cmp(m.MaxFeePerGas) > 0 {
		if baseFeePerGas.Cmp(m.MaxFeePerGas) > 0 {
			return Value0, fmt.Errorf(
				"max fee per gas is less than base fee per gas: %s < %s", m.MaxFeePerGas, baseFeePerGas)
		}
		gasPrice = m.MaxFeePerGas
	}
	return gasPrice, nil
}

func (m InternalTransactionPayload) ToTransaction(from Address, seqno Seqno) *Transaction {
	txn := &Transaction{
		TransactionDigest: TransactionDigest{
			Flags: TransactionFlagsFromKind(true, m.Kind),
			To:    m.To,
			Data:  m.Data,
			FeePack: FeePack{
				FeeCredit:            m.FeeCredit,
				MaxPriorityFeePerGas: NewZeroValue(),
				MaxFeePerGas:         NewZeroValue(),
			},
			Seqno: seqno,
		},
		RefundTo:  m.RefundTo,
		BounceTo:  m.BounceTo,
		From:      from,
		Value:     m.Value,
		Token:     m.Token,
		RequestId: m.RequestId,
	}
	if m.Bounce {
		txn.Flags.SetBit(TransactionFlagBounce)
	}

	return txn
}

func (m *ExternalTransaction) Hash() common.Hash {
	return ToShardedHash(common.MustKeccak(m), m.To.ShardId())
}

func (m *ExternalTransaction) SigningHash() (common.Hash, error) {
	transactionDigest := TransactionDigest{
		Flags:   TransactionFlagsFromKind(false, m.Kind),
		FeePack: m.FeePack,
		Seqno:   m.Seqno,
		To:      m.To,
		Data:    m.Data,
		ChainId: m.ChainId,
	}

	return common.Keccak(&transactionDigest)
}

func (m ExternalTransaction) ToTransaction() *Transaction {
	return &Transaction{
		TransactionDigest: TransactionDigest{
			Flags:   TransactionFlagsFromKind(false, m.Kind),
			To:      m.To,
			ChainId: m.ChainId,
			Seqno:   m.Seqno,
			Data:    m.Data,
			FeePack: m.FeePack,
		},
		From:      m.To,
		Signature: m.AuthData,
	}
}

func (m *Transaction) SigningHash() (common.Hash, error) {
	return common.Keccak(&m.TransactionDigest)
}

func (m *ExternalTransaction) Sign(key *ecdsa.PrivateKey) error {
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

func NewTransactionFlags(flags ...int) TransactionFlags {
	return TransactionFlags{NewBitFlags[uint8](flags...)}
}

func TransactionFlagsFromKind(internal bool, kind TransactionKind) TransactionFlags {
	flags := make([]int, 0, 2)
	if internal {
		flags = append(flags, TransactionFlagInternal)
	}
	switch kind {
	case DeployTransactionKind:
		flags = append(flags, TransactionFlagDeploy)
	case RefundTransactionKind:
		flags = append(flags, TransactionFlagRefund)
	case ResponseTransactionKind:
		flags = append(flags, TransactionFlagResponse)
	case ExecutionTransactionKind: // do nothing
	}
	return NewTransactionFlags(flags...)
}

func (m TransactionFlags) String() string {
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

func (m TransactionFlags) MarshalJSON() ([]byte, error) {
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

func (m *TransactionFlags) UnmarshalJSON(data []byte) error {
	m.Clear()
	var s []string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, v := range s {
		switch v {
		case "Internal":
			m.SetBit(TransactionFlagInternal)
		case "Deploy":
			m.SetBit(TransactionFlagDeploy)
		case "Refund":
			m.SetBit(TransactionFlagRefund)
		case "Bounce":
			m.SetBit(TransactionFlagBounce)
		case "Response":
			m.SetBit(TransactionFlagResponse)
		}
	}
	return nil
}

func (m TransactionFlags) IsInternal() bool {
	return m.GetBit(TransactionFlagInternal)
}

func (m TransactionFlags) IsDeploy() bool {
	return m.GetBit(TransactionFlagDeploy)
}

func (m TransactionFlags) IsRefund() bool {
	return m.GetBit(TransactionFlagRefund)
}

func (m TransactionFlags) IsBounce() bool {
	return m.GetBit(TransactionFlagBounce)
}

func (m TransactionFlags) IsResponse() bool {
	return m.GetBit(TransactionFlagResponse)
}

type TxnWithHash struct {
	*Transaction
	hash common.Hash
}

func NewTxnWithHash(txn *Transaction) *TxnWithHash {
	return &TxnWithHash{
		Transaction: txn,
		hash:        txn.Hash(),
	}
}

func (m *TxnWithHash) Hash() common.Hash {
	return m.hash
}
