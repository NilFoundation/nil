package tracer

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Until the hash function question is not finaized yet https://github.com/NilFoundation/placeholder/issues/205
// this wrapper implements adjustment of the cluster logic to current placeholder expectations
// TODO needs to be cleaned up!!!
func getCodeHash(code types.Code) common.Hash {
	if len(code) == 0 {
		return common.EmptyHash
	}
	return common.BytesToHash(crypto.Keccak256(code))
	// return code.Hash()
}
