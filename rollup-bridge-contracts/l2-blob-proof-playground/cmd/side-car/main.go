package main

import (
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
	side_car "l2-blob-proof-playground/side-car"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// go run cmd/side-car/main.go
func main() {

	var blob, err = blobutil.GenerateRandomBlob()

	if err != nil {
		panic(err)
	}

	var sidecar *gethTypes.BlobTxSidecar
	sidecar, sideCarErr := side_car.MakeSidecar(blob)

	if sideCarErr != nil {
		panic(err)
	}

	if sidecar == nil {
		panic("sidecar is nil")
	}

	fmt.Printf("Generated sidecar:\n")
	fmt.Printf("Blobs:\n")
	for i, b := range sidecar.Blobs {
		fmt.Printf("  Blob %d: %x\n", i, b)
	}
	fmt.Printf("Commitments:\n")
	for i, c := range sidecar.Commitments {
		fmt.Printf("  Commitment %d: %x\n", i, c)
	}
	fmt.Printf("Proofs:\n")
	for i, p := range sidecar.Proofs {
		fmt.Printf("  Proof %d: %x\n", i, p)
	}

	//get the blobHashes of the sidecar blobs
	var versionedHashes []common.Hash = sidecar.BlobHashes()
	fmt.Printf("Blob hashes:%v \n", versionedHashes)
}
