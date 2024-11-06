package common

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func TestParseCallArgument(t *testing.T) {
	t.Parallel()

	val, ok := new(big.Int).SetString("1234567890123456789012345678901234567890", 10)
	require.True(t, ok)

	tests := []struct {
		name      string
		arg       string
		tp        abi.Type
		want      any
		expectErr bool
	}{
		{
			name: "Parse int",
			arg:  "123",
			tp:   abi.Type{T: abi.IntTy, Size: 64},
			want: int64(123),
		},
		{
			name: "Parse uint",
			arg:  "123",
			tp:   abi.Type{T: abi.UintTy, Size: 64},
			want: uint64(123),
		},
		{
			name: "Parse big int",
			arg:  "1234567890123456789012345678901234567890",
			tp:   abi.Type{T: abi.IntTy, Size: 256},
			want: val,
		},
		{
			name: "Parse string",
			arg:  "hello",
			tp:   abi.Type{T: abi.StringTy},
			want: "hello",
		},
		{
			name: "Parse bytes32",
			arg:  "0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
			tp:   abi.Type{T: abi.FixedBytesTy, Size: 32},
			want: [32]uint8(hexutil.MustDecode("0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")),
		},
		{
			name:      "Parse bytes32",
			arg:       "0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeffffffffff",
			tp:        abi.Type{T: abi.FixedBytesTy, Size: 32},
			expectErr: true,
		},
		{
			name: "Parse bytes",
			arg:  "0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
			tp:   abi.Type{T: abi.BytesTy},
			want: hexutil.MustDecode("0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
		},
		{
			name: "Parse bool true",
			arg:  "true",
			tp:   abi.Type{T: abi.BoolTy},
			want: true,
		},
		{
			name: "Parse address",
			arg:  "0x1234567890123456789012345678901234567890",
			tp:   abi.Type{T: abi.AddressTy},
			want: types.HexToAddress("0x1234567890123456789012345678901234567890"),
		},
		{
			name:      "Parse invalid int",
			arg:       "invalid",
			tp:        abi.Type{T: abi.IntTy, Size: 64},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCallArgument(tt.arg, tt.tp)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}
