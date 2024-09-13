package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func BenchmarkJSONMarshal(b *testing.B) {
	addr := types.GenerateRandomAddress(0)
	callArgs := CallArgs{
		Flags:     types.NewMessageFlags(),
		From:      &addr,
		To:        addr,
		FeeCredit: types.NewValueFromUint64(123),
		Value:     types.NewValueFromUint64(321),
		Seqno:     types.Seqno(10),
		Data:      &hexutil.Bytes{'a', 'b', 'c'},
		ChainId:   types.ChainId(0),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(callArgs)
		if err != nil {
			b.Fatalf("Failed to marshal: %v", err)
		}
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	addr := types.GenerateRandomAddress(0)
	callArgs := CallArgs{
		Flags:     types.NewMessageFlags(),
		From:      &addr,
		To:        addr,
		FeeCredit: types.NewValueFromUint64(123),
		Value:     types.NewValueFromUint64(321),
		Seqno:     types.Seqno(10),
		Data:      &hexutil.Bytes{'a', 'b', 'c'},
		ChainId:   types.ChainId(0),
	}

	data, err := json.Marshal(callArgs)
	if err != nil {
		b.Fatalf("Failed to marshal: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var callArgs CallArgs
		err := json.Unmarshal(data, &callArgs)
		if err != nil {
			b.Fatalf("Failed to unmarshal: %v", err)
		}
	}
}
