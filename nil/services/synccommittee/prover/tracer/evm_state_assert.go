//go:build assert

package tracer

import (
	"crypto/sha1"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/tracing"
)

func assertEVMStateConsistent(pc uint64, scope tracing.OpContext, returnData []byte) func() {
	hashMutableData := func() common.Hash {
		hash := sha1.New()

		// add every mutable value from current EVM to hash state
		// tracer must not do any changes to the EVM state
		for _, word := range scope.StackData() {
			hash.Write(word.Bytes())
		}
		if callValue := scope.CallValue(); callValue != nil {
			hash.Write(scope.CallValue().Bytes())
		}
		hash.Write(scope.MemoryData())
		hash.Write(returnData)
		result := hash.Sum(nil)
		return common.BytesToHash(result)
	}
	storedState := hashMutableData()

	return func() {
		actualState := hashMutableData()
		check.PanicIfNotf(storedState == actualState, "got corrupted EVM state (%s vs %s) on pc %d tracing", storedState, actualState, pc)
	}
}
