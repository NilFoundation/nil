package main

import (
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
	proof "l2-blob-proof-playground/proof"
	side_car "l2-blob-proof-playground/side-car"

	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

// go run cmd/batch-data-proof/main.go
func main() {

	// generate sidecar with 3 blobs
	blobs, err := GenerateSideCarWitMultipleBlobs(3)
	if err != nil {
		panic(err)
	}

	var dataProofResponses []*proof.DataProofResponse
	var dataProofsErr error

	// generate dataProof for all the blobs in the sidecar
	dataProofResponses, dataProofsErr = proof.GenerateDataProofFromSidecar(blobs)

	// assert if the dataProof is valid for all the blobs in the sidecar
	if dataProofsErr != nil {
		panic(err)
	}

	fmt.Printf("\n")

	for dataProofIndex, dataProofResponse := range dataProofResponses {
		fmt.Printf("\n\n Generated Data Proof for blob %d:\n", dataProofIndex)
		if dataProofResponse == nil || len(dataProofResponse.DataProof) == 0 {
			panic(fmt.Sprintf("Data Proof for blob %d is nil\n", dataProofIndex))
		}
		fmt.Printf("  DataProof: %x\n\n", dataProofResponse.DataProof)
		fmt.Printf(" DataProof components for blob %d are: \n", dataProofIndex)
		fmt.Printf("  Point: %x\n", dataProofResponse.Point)
		fmt.Printf("  Claim: %x\n", dataProofResponse.Claim)
		fmt.Printf("  KZGCommitment: %x\n", dataProofResponse.KZGCommitment)
		fmt.Printf("  KZGProof: %x\n", dataProofResponse.KZGProof)
		fmt.Printf("  VersionedHash: %x\n", dataProofResponse.VersionedHash)
	}
	fmt.Printf("\n")

}

func GenerateSideCarWitMultipleBlobs(blobCount int8) (*gethTypes.BlobTxSidecar, error) {
	var blobs []*kzg4844.Blob
	var blob *kzg4844.Blob
	var err error
	for i := 0; i < int(blobCount); i++ {
		blob, err = blobutil.GenerateRandomBlob()
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}
	sidecar, err := side_car.MakeSidecar(blobs...)
	if err != nil {
		return nil, err
	}
	return sidecar, nil
}
