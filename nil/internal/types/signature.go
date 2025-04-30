package types

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type Signature = hexutil.Bytes

type BlsSignature = hexutil.Bytes

type BlsAggregateSignature struct {
	Sig  hexutil.Bytes `json:"sig" yaml:"sig"`
	Mask hexutil.Bytes `json:"mask" yaml:"mask"`
}

func (b BlsAggregateSignature) String() string {
	return fmt.Sprintf("BlsAggregateSignature{Sig: %x, Mask: %x}", b.Sig, b.Mask)
}
