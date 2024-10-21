package types

import (
	"database/sql/driver"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type SmartContract struct {
	Address          Address
	Initialised      bool
	Balance          Value `ssz-size:"32"`
	CurrencyRoot     common.Hash
	StorageRoot      common.Hash
	CodeHash         common.Hash
	AsyncContextRoot common.Hash
	Seqno            Seqno
	ExtSeqno         Seqno
	RequestId        uint64
}

type CurrencyId Address

func (c CurrencyId) String() string {
	return Address(c).String()
}

func (c CurrencyId) MarshalText() ([]byte, error) {
	return Address(c).MarshalText()
}

func (c *CurrencyId) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("CurrencyId", input, c[:])
}

type CurrencyBalance struct {
	Currency CurrencyId `json:"id" ssz-size:"20" abi:"id"`
	Balance  Value      `json:"value" ssz-size:"32" abi:"amount"`
}

func (currency CurrencyBalance) Value() (driver.Value, error) {
	return []interface{}{currency.Currency, currency.Balance.ToBig()}, nil
}

func CurrencyIdForAddress(a Address) *CurrencyId {
	r := CurrencyId(a)
	return &r
}

// interfaces
var (
	_ driver.Valuer       = new(CurrencyBalance)
	_ common.Hashable     = new(SmartContract)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	return common.MustPoseidonSSZ(s)
}

type CurrenciesMap = map[CurrencyId]Value

type RPCCurrenciesMap = CurrenciesMap
