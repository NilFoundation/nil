package types

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/holiman/uint256"
)

type Storage map[common.Hash]uint256.Int

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}
