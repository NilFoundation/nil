package rpc

import (
	"bytes"
	"testing"
)

func TestTypescriptGeneration(t *testing.T) {
	t.Parallel()

	// create buffer to write to test

	s, err := ExportTypescriptTypes()
	if err != nil {
		t.Errorf("Failed to export typescript types")
	}

	// check if the buffer is empty
	if len(s) == 0 {
		t.Errorf("Expected buffer to not be empty")
	}

	// check if the buffer contains the expected string
	if !bytes.Contains(s, []byte("interface EthAPI")) {
		t.Errorf("Expected buffer to contain interface EthAPI")
	}
}
