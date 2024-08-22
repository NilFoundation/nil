// Copyright 2017 The go-ethereum Authors
// (original work)
// Copyright 2024 The Erigon Authors
// (modifications)
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.
//
//nolint:scopelint
package abi

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

// TestPack tests the general pack/unpack tests in packing_test.go
func TestPack(t *testing.T) { //nolint:tparallel
	t.Parallel()

	for i, test := range packUnpackTests { //nolint:paralleltest
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			encb, err := hex.DecodeString(test.packed)
			if err != nil {
				t.Fatalf("invalid hex %s: %v", test.packed, err)
			}
			inDef := fmt.Sprintf(`[{ "name" : "method", "type": "function", "inputs": %s}]`, test.def)
			inAbi, err := JSON(strings.NewReader(inDef))
			if err != nil {
				t.Fatalf("invalid ABI definition %s, %v", inDef, err)
			}
			var packed []byte
			packed, err = inAbi.Pack("method", test.unpacked)
			if err != nil {
				t.Fatalf("test %d (%v) failed: %v", i, test.def, err)
			}
			if !reflect.DeepEqual(packed[4:], encb) {
				t.Errorf("test %d (%v) failed: expected %v, got %v", i, test.def, encb, packed[4:])
			}
		})
	}
}

func TestMethodPack(t *testing.T) {
	t.Parallel()

	abi, err := JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}

	sig := abi.Methods["slice"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)

	packed, err := abi.Pack("slice", []uint32{1, 2})
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	addrA, addrB := types.Address{1}, types.Address{2}
	sig = abi.Methods["sliceAddress"].ID
	sig = append(sig, common.LeftPadBytes([]byte{32}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrA[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrB[:], 32)...)

	packed, err = abi.Pack("sliceAddress", []types.Address{addrA, addrB})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	addrC, addrD := types.Address{3}, types.Address{4}
	sig = abi.Methods["sliceMultiAddress"].ID
	sig = append(sig, common.LeftPadBytes([]byte{64}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{160}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrA[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrB[:], 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrC[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrD[:], 32)...)

	packed, err = abi.Pack("sliceMultiAddress", []types.Address{addrA, addrB}, []types.Address{addrC, addrD})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["slice256"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)

	packed, err = abi.Pack("slice256", []*big.Int{big.NewInt(1), big.NewInt(2)})
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	a := [2][2]*big.Int{{big.NewInt(1), big.NewInt(1)}, {big.NewInt(2), big.NewInt(0)}}
	sig = abi.Methods["nestedArray"].ID
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0xa0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes(addrC[:], 32)...)
	sig = append(sig, common.LeftPadBytes(addrD[:], 32)...)
	packed, err = abi.Pack("nestedArray", a, []types.Address{addrC, addrD})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["nestedArray2"].ID
	sig = append(sig, common.LeftPadBytes([]byte{0x20}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x40}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x80}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	packed, err = abi.Pack("nestedArray2", [2][]uint8{{1}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}

	sig = abi.Methods["nestedSlice"].ID
	sig = append(sig, common.LeftPadBytes([]byte{0x20}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x02}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0x40}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{0xa0}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{1}, 32)...)
	sig = append(sig, common.LeftPadBytes([]byte{2}, 32)...)
	packed, err = abi.Pack("nestedSlice", [][]uint8{{1, 2}, {1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, sig) {
		t.Errorf("expected %x got %x", sig, packed)
	}
}

func TestPackNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value  reflect.Value
		packed []byte
	}{
		// Protocol limits
		{reflect.ValueOf(0), hexutil.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000000")},
		{reflect.ValueOf(1), hexutil.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")},
		{reflect.ValueOf(-1), hexutil.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")},

		// Type corner cases
		{reflect.ValueOf(uint8(math.MaxUint8)), hexutil.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000ff")},
		{reflect.ValueOf(uint16(math.MaxUint16)), hexutil.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000ffff")},
		{reflect.ValueOf(uint32(math.MaxUint32)), hexutil.Hex2Bytes("00000000000000000000000000000000000000000000000000000000ffffffff")},
		{reflect.ValueOf(uint64(math.MaxUint64)), hexutil.Hex2Bytes("000000000000000000000000000000000000000000000000ffffffffffffffff")},

		{reflect.ValueOf(int8(math.MaxInt8)), hexutil.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000007f")},
		{reflect.ValueOf(int16(math.MaxInt16)), hexutil.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000007fff")},
		{reflect.ValueOf(int32(math.MaxInt32)), hexutil.Hex2Bytes("000000000000000000000000000000000000000000000000000000007fffffff")},
		{reflect.ValueOf(int64(math.MaxInt64)), hexutil.Hex2Bytes("0000000000000000000000000000000000000000000000007fffffffffffffff")},

		{reflect.ValueOf(int8(math.MinInt8)), hexutil.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80")},
		{reflect.ValueOf(int16(math.MinInt16)), hexutil.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000")},
		{reflect.ValueOf(int32(math.MinInt32)), hexutil.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000000")},
		{reflect.ValueOf(int64(math.MinInt64)), hexutil.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffff8000000000000000")},
	}
	for i, tt := range tests {
		packed := packNum(tt.value)
		if !bytes.Equal(packed, tt.packed) {
			t.Errorf("test %d: pack mismatch: have %x, want %x", i, packed, tt.packed)
		}
	}
}

func TestPackUint256(t *testing.T) { //nolint:tparallel
	t.Parallel()

	type Uint256Struct struct {
		Number types.Uint256
	}

	abiJsonStr := `[{
		"inputs": [
			{
				"components": [
					{
						"internalType": "uint256",
						"name": "Number",
						"type": "uint256"
					}
				],
				"name": "param",
				"type": "tuple"
			}
		],
		"name": "test",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "Number",
				"type": "uint256"
			}
		],
		"type": "function"
    }]`
	inAbi, err := JSON(strings.NewReader(abiJsonStr))
	require.NoError(t, err)

	uStruct := Uint256Struct{*types.NewUint256(123)}
	data, err := inAbi.Pack("test", &uStruct)
	require.NoError(t, err)

	t.Run("Unpack Uint256", func(t *testing.T) { //nolint:paralleltest
		unpacked, err := inAbi.Unpack("test", data[4:])
		require.NoError(t, err)

		u := ConvertType(unpacked[0], new(types.Uint256))
		num, ok := u.(*types.Uint256)
		require.True(t, ok)
		require.Equal(t, uStruct.Number, *num)
	})

	t.Run("Unpack Uint256Struct", func(t *testing.T) { //nolint:paralleltest
		unpacked, err := inAbi.Methods["test"].Inputs.Unpack(data[4:])
		require.NoError(t, err)

		as := ConvertType(unpacked[0], new(Uint256Struct))
		obj, ok := as.(*Uint256Struct)
		require.True(t, ok)

		require.Equal(t, uStruct.Number, obj.Number)
	})
}
