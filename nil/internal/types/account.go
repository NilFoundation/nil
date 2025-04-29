package types

import (
	"database/sql/driver"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/serialization"
)

type SmartContract struct {
	Address          Address
	Balance          Value `ssz-size:"32"`
	TokenRoot        common.Hash
	StorageRoot      common.Hash
	CodeHash         common.Hash
	AsyncContextRoot common.Hash
	Seqno            Seqno
	ExtSeqno         Seqno
}

func (s *SmartContract) UnmarshalNil(buf []byte) error {
	return s.UnmarshalSSZ(buf)
}

func (s SmartContract) MarshalNil() ([]byte, error) {
	return s.MarshalSSZ()
}

type TokenId Address

func (c TokenId) String() string {
	return Address(c).String()
}

func (c TokenId) MarshalText() ([]byte, error) {
	return Address(c).MarshalText()
}

func (c *TokenId) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("TokenId", input, c[:])
}

type TokenBalance struct {
	Token   TokenId `json:"id" ssz-size:"20" abi:"id"`
	Balance Value   `json:"value" ssz-size:"32" abi:"amount"`
}

func (token TokenBalance) Value() (driver.Value, error) {
	return []any{token.Token, token.Balance.ToBig()}, nil
}

func TokenIdForAddress(a Address) *TokenId {
	r := TokenId(a)
	return &r
}

// interfaces
var (
	_ driver.Valuer                = new(TokenBalance)
	_ common.Hashable              = new(SmartContract)
	_ serialization.NilMarshaler   = new(Block)
	_ serialization.NilUnmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	return common.MustKeccak(s)
}

type TokensMap = map[TokenId]Value

type RPCTokensMap = TokensMap
