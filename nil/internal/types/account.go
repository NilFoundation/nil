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
	TokenRoot        common.Hash
	StorageRoot      common.Hash
	CodeHash         common.Hash
	AsyncContextRoot common.Hash
	Seqno            Seqno
	ExtSeqno         Seqno
	RequestId        uint64
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
	return []interface{}{token.Token, token.Balance.ToBig()}, nil
}

func TokenIdForAddress(a Address) *TokenId {
	r := TokenId(a)
	return &r
}

// interfaces
var (
	_ driver.Valuer       = new(TokenBalance)
	_ common.Hashable     = new(SmartContract)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (s *SmartContract) Hash() common.Hash {
	return common.MustPoseidonSSZ(s)
}

type TokensMap = map[TokenId]Value

type RPCTokensMap = TokensMap

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path account.go -include ../../common/length.go,transaction.go,address.go,value.go,code.go,shard.go,bloom.go,log.go,../../common/hash.go --objs SmartContract,TokenBalance
