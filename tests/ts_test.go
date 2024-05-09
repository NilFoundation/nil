package tests

import (
	"bytes"
	"testing"

	"github.com/NilFoundation/nil/rpc"
)

func TestTypescriptGeneration(t *testing.T) {

	// create buffer to write to test
	s := &bytes.Buffer{}
	rpc.ExportTypescriptTypes(s)

	// check if the buffer is empty
	if s.Len() == 0 {
		t.Errorf("Expected buffer to not be empty")
	}

	// check if the buffer contains the expected string
	if !bytes.Contains(s.Bytes(), []byte("interface EthAPI")) {
		t.Errorf("Expected buffer to contain interface EthAPI")
	}

}
