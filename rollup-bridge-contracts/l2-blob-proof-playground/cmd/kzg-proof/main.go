package main

import (
	"encoding/hex"
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
	"l2-blob-proof-playground/proof"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

// go run cmd/kzg-proof/main.go
func main() {

	var blob *kzg4844.Blob
	var blobGenerationError error

	blob, blobGenerationError = blobutil.GenerateRandomBlob()

	if blobGenerationError != nil {
		panic(blobGenerationError)
	}

	var point kzg4844.Point
	var claim kzg4844.Claim
	var kzgProof kzg4844.Proof
	var err error
	var kzgProofResponse *proof.KZGProofResponse

	kzgProofResponse, err = proof.GenerateKZGProofFromBlob(*blob)

	if err != nil {
		panic(err)
	}

	point = kzgProofResponse.Point
	claim = kzgProofResponse.Claim
	kzgProof = kzgProofResponse.Proof

	fmt.Printf("Generated KZG proof:\n")
	fmt.Printf("versionedHash: %s\n", hex.EncodeToString(point[:]))
	fmt.Printf("point: %s\n", hex.EncodeToString(claim[:]))
	fmt.Printf("kzgProof: %s\n", hex.EncodeToString(kzgProof[:]))
}
