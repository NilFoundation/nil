package rpc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeSockPathImpl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filename    string
		shouldBeCut bool
	}{
		{"ShortNameZ", "short.sock", false},
		{"LongName80characters-----------------------------------------------------------Z", "short.sock", false},
		{"LongName80characters-----------------------------------------------------------Z", "loooooong.sock", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := makeSockPathImpl(
				tt.name,
				tt.filename,
				func(dir string, err error) {
					require.NoError(t, err)
				})
			require.Lessf(t, len(path), 108, "Generated path should be less than 108 characters: %s", path)

			lastDirPrefixSymbol := path[strings.LastIndex(path, "_")-1]
			if !tt.shouldBeCut {
				require.EqualValues(t, 'Z', lastDirPrefixSymbol)
			} else {
				require.NotEqualValues(t, 'Z', lastDirPrefixSymbol)
			}
		})
	}
}
