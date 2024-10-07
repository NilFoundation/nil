package testaide

import (
	"crypto/rand"

	"github.com/NilFoundation/nil/nil/common"
)

func GenerateRandomBlockHash() common.Hash {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return common.BytesToHash(randomBytes)
}
