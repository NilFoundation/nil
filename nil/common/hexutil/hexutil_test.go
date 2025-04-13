package hexutil

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type marshalTest struct {
	input any
	want  string
}

type unmarshalTest struct {
	input        string
	want         any
	wantErr      error // if set, decoding must fail on any platform
	wantErr32bit error // if set, decoding must fail on 32bit platforms (used for Uint tests)
}

var (
	encodeBigTests = []marshalTest{
		{bigFromString("0"), "0x0"},
		{bigFromString("1"), "0x1"},
		{bigFromString("ff"), "0xff"},
		{bigFromString("112233445566778899aabbccddeeff"), "0x112233445566778899aabbccddeeff"},
		{bigFromString("80a7f2c1bcc396c00"), "0x80a7f2c1bcc396c00"},
		{bigFromString("-80a7f2c1bcc396c00"), "-0x80a7f2c1bcc396c00"},
	}

	encodeUint64Tests = []marshalTest{
		{uint64(0), "0x0"},
		{uint64(1), "0x1"},
		{uint64(0xff), "0xff"},
		{uint64(0x1122334455667788), "0x1122334455667788"},
	}

	encodeUintTests = []marshalTest{
		{uint(0), "0x0"},
		{uint(1), "0x1"},
		{uint(0xff), "0xff"},
		{uint(0x11223344), "0x11223344"},
	}

	decodeUint64Tests = []unmarshalTest{
		// invalid
		{input: `0`, wantErr: ErrMissingPrefix},
		{input: `0x`, wantErr: ErrEmptyNumber},
		{input: `0x01`, wantErr: ErrLeadingZero},
		{input: `0xfffffffffffffffff`, wantErr: ErrUint64Range},
		{input: `0xx`, wantErr: ErrSyntax},
		{input: `0x1zz01`, wantErr: ErrSyntax},
		// valid
		{input: `0x0`, want: uint64(0)},
		{input: `0x2`, want: uint64(0x2)},
		{input: `0x2F2`, want: uint64(0x2f2)},
		{input: `0X2F2`, want: uint64(0x2f2)},
		{input: `0x1122aaff`, want: uint64(0x1122aaff)},
		{input: `0xbbb`, want: uint64(0xbbb)},
		{input: `0xffffffffffffffff`, want: uint64(0xffffffffffffffff)},
	}
)

func TestEncodeBig(t *testing.T) {
	t.Parallel()

	for idx, test := range encodeBigTests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			t.Parallel()

			in, ok := test.input.(*big.Int)
			require.True(t, ok)
			require.Equal(t, test.want, EncodeBig(in))
		})
	}
}

func TestEncodeUint64(t *testing.T) {
	t.Parallel()

	for idx, test := range encodeUint64Tests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			t.Parallel()

			in, ok := test.input.(uint64)
			require.True(t, ok)
			require.Equal(t, test.want, EncodeUint64(in))
		})
	}
}

func TestDecodeUint64(t *testing.T) {
	t.Parallel()

	for idx, test := range decodeUint64Tests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			t.Parallel()

			dec, err := DecodeUint64(test.input)
			checkError(t, test.input, err, test.wantErr)
			if test.want != nil {
				require.Equal(t, test.want, dec)
			}
		})
	}
}
