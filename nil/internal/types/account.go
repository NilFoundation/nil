package types

import (
	"database/sql/driver"
	"math/big"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type SmartContract struct {
	Address      Address
	Initialised  bool
	Balance      Value `ssz-size:"32"`
	CurrencyRoot common.Hash
	StorageRoot  common.Hash
	CodeHash     common.Hash
	Seqno        Seqno
	ExtSeqno     Seqno
}

type CurrencyId common.Hash

func (c CurrencyId) String() string {
	return common.Hash(c).String()
}

type CurrencyBalance struct {
	Currency CurrencyId `json:"id" ssz-size:"32" abi:"id"`
	Balance  Value      `json:"value" ssz-size:"32" abi:"amount"`
}

// CurrencyBalanceAbiCompatible is a struct same as CurrencyBalance, but compatible with the eth ABI
// TODO: merge with CurrencyBalance
type CurrencyBalanceAbiCompatible struct {
	Currency *big.Int `abi:"id"`
	Balance  *big.Int `abi:"amount"`
}

func (currency CurrencyBalance) Value() (driver.Value, error) {
	return []interface{}{currency.Currency, currency.Balance.ToBig()}, nil
}

func CurrencyIdForAddress(a Address) *CurrencyId {
	c := new(CurrencyId)
	copy(c[12:], a.Bytes())
	return c
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

type CurrenciesMap = map[string]Value

type RPCCurrenciesMap = map[string]*hexutil.Big

func ToCurrenciesMap(m RPCCurrenciesMap) CurrenciesMap {
	return common.TransformMap(m, func(k string, v *hexutil.Big) (string, Value) {
		return k, NewValueFromBigMust(v.ToInt())
	})
}
