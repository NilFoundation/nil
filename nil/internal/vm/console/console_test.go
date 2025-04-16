package console

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func TestLogs(t *testing.T) {
	t.Parallel()

	uint256Ty, err := abi.NewType("uint256", "", nil)
	require.NoError(t, err)
	addressTy, err := abi.NewType("address", "", nil)
	require.NoError(t, err)
	stringTy, err := abi.NewType("string", "", nil)
	require.NoError(t, err)

	tests := []struct {
		name       string
		format     string
		params     []abi.Type
		args       []any
		result     string
		expectFail bool
	}{
		{
			name:   "test 1",
			format: "test args: int=%_, address=%_, string=%_",
			params: []abi.Type{stringTy, uint256Ty, addressTy, stringTy},
			args:   []any{big.NewInt(123), types.HexToAddress("0x987654"), "hello world"},
			result: "test args: int=123, address=0x0000000000000000000000000000000000987654, string=hello world",
		},
		{
			name:   "test 2",
			format: "int=%_, int=%x, int=%_--",
			params: []abi.Type{stringTy, uint256Ty, uint256Ty, uint256Ty},
			args:   []any{big.NewInt(123), big.NewInt(456), big.NewInt(789)},
			result: "int=123, int=0x1c8, int=789--",
		},
		{
			name:   "test 3",
			format: "int=%%_, int=%%%x, int=%t",
			params: []abi.Type{stringTy, uint256Ty},
			args:   []any{big.NewInt(123)},
			result: "int=%_, int=%0x7b, int=%t",
		},
		{
			name:       "test 4: no format arg",
			format:     "",
			params:     []abi.Type{uint256Ty},
			args:       []any{big.NewInt(123)},
			expectFail: true,
		},
		{
			name:   "test 5",
			format: "%x",
			params: []abi.Type{stringTy, addressTy},
			args:   []any{types.HexToAddress("0x1234")},
			result: "0x0000000000000000000000000000000000001234",
		},
		{
			name:   "test 6",
			format: "test",
			params: []abi.Type{stringTy},
			args:   nil,
			result: "test",
		},
		{
			name:       "test 7: not enough args",
			format:     "test: %_",
			params:     []abi.Type{stringTy},
			args:       nil,
			expectFail: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			params := make(abi.Arguments, len(test.params))
			for i, param := range test.params {
				params[i] = abi.Argument{
					Name: param.String(),
					Type: param,
				}
			}

			method := abi.NewMethod("log", "log", abi.Function, "", false, false, params, nil)
			var args []any
			if test.format != "" {
				args = append([]any{test.format}, test.args...)
			} else {
				args = test.args
			}
			data, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			data = append(method.ID, data...)

			str, err := ProcessLog(data)
			if test.expectFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.result, str)
			}
		})
	}
}
