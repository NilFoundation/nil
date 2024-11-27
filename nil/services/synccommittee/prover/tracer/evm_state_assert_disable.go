//go:build !assert

package tracer

import "github.com/NilFoundation/nil/nil/internal/tracing"

func assertEVMStateConsistent(
	_ uint64, // pc
	_ tracing.OpContext, // scope
	_ []byte, // returnData
) func() {
	return func() {}
}
