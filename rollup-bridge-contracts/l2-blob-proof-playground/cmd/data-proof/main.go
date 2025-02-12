package main

import (
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
	"l2-blob-proof-playground/proof"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

// go run cmd/data-proof/main.go
func main() {
	// generate randomBlob
	var blob *kzg4844.Blob
	var blobGenerationError error

	blob, blobGenerationError = blobutil.GenerateRandomBlob()

	if blobGenerationError != nil {
		panic(blobGenerationError)
	}

	var dataProofResponse *proof.DataProofResponse
	var dataProofGenerationError error

	// generate dataProof from randomBlob
	dataProofResponse, dataProofGenerationError = proof.GenerateDataProofFromBlob(blob)

	if dataProofGenerationError != nil {
		panic(dataProofGenerationError)
	}

	if dataProofResponse == nil {
		panic("dataProofResponse is nil")
	}

	if len(dataProofResponse.DataProof) == 0 {
		panic("dataProofResponse.DataProof is empty")
	}

	fmt.Printf("Generated Data Proof:\n")
	fmt.Printf("Data Proof: %x\n", dataProofResponse.DataProof)
}
