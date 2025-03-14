package l1

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

type EthClient interface {
	bind.ContractBackend
	bind.ContractFilterer
	bind.ContractTransactor
}
