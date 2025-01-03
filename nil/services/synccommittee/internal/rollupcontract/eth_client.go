package rollupcontract

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

type EthClient interface {
	bind.ContractBackend
	ChainID(ctx context.Context) (*big.Int, error)
}
