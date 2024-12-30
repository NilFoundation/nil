package types

import (
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
)

func TestBloom(t *testing.T) {
	t.Parallel()

	positive := []string{
		"testtest",
		"test",
		"hallo",
		"other",
	}
	negative := []string{
		"tes",
		"lo",
	}

	var bloom Bloom
	for _, data := range positive {
		bloom.Add([]byte(data))
	}

	for _, data := range positive {
		if !bloom.Test([]byte(data)) {
			t.Error("expected", data, "to test true")
		}
	}
	for _, data := range negative {
		if bloom.Test([]byte(data)) {
			t.Error("did not expect", data, "to test true")
		}
	}
}

// TestBloomExtensively does some more thorough tests
func TestBloomExtensively(t *testing.T) {
	t.Parallel()

	exp := common.HexToHash("09f96160f0da75ea63ed9ff270f994de890e0dce6e6fb532dcc332ab3b90ddbd")
	var b Bloom
	// Add 100 "random" things
	for i := range 100 {
		data := fmt.Sprintf("xxxxxxxxxx data %d yyyyyyyyyyyyyy", i)
		b.Add([]byte(data))
	}
	got := common.PoseidonHash(b.Bytes())
	if got != exp {
		t.Errorf("Got %x, exp %x", got, exp)
	}
	var b2 Bloom
	b2.SetBytes(b.Bytes())
	got2 := common.PoseidonHash(b2.Bytes())
	if got != got2 {
		t.Errorf("Got %x, exp %x", got, got2)
	}
}

func BenchmarkBloom9(b *testing.B) {
	test := []byte("testestestest")
	for range b.N {
		Bloom9(test)
	}
}

func BenchmarkBloom9Lookup(b *testing.B) {
	toTest := []byte("testtest")
	bloom := new(Bloom)
	for range b.N {
		bloom.Test(toTest)
	}
}
