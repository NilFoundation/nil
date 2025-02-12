package proof

import (
	"encoding/hex"
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type DataProofResponse struct {
	Point         kzg4844.Point
	Claim         [32]byte
	KZGCommitment kzg4844.Commitment
	KZGProof      kzg4844.Proof
	DataProof     []byte
	VersionedHash common.Hash
}

func GenerateDataProofFromBlob(blob *kzg4844.Blob) (*DataProofResponse, error) {
	kzgProof, err := GenerateKZGProofFromBlob(*blob)
	if err != nil {
		panic("Failed to generate KZG proof from the blob")
	}

	var claimArray [32]byte
	copy(claimArray[:], kzgProof.Claim[:])
	claimHex := hex.EncodeToString(claimArray[:])
	fmt.Printf("claim: %s\n", claimHex)

	kzgProofHex := hex.EncodeToString(kzgProof.Proof[:])
	fmt.Printf("kzgProof: %s\n", kzgProofHex)

	var kzgCommitment kzg4844.Commitment
	var kzgCommitmentGenerationError error

	kzgCommitment, kzgCommitmentGenerationError = kzg4844.BlobToCommitment(blob)

	if kzgCommitmentGenerationError != nil {
		fmt.Printf("failed to generate commitment from the blob")
		return nil, kzgCommitmentGenerationError
	}

	var versionedHash = blobutil.GenerateVersionedHashFromCommitment(kzgCommitment)

	var dataProof = blobDataProofFromValues(kzgProof.Point, claimArray, kzgCommitment, kzgProof.Proof)

	dataProofHex := hex.EncodeToString(dataProof)
	fmt.Printf("blobDataProof: %s\n", dataProofHex)

	err = kzg4844.VerifyProof(kzgCommitment, kzgProof.Point, kzgProof.Claim, kzgProof.Proof)
	if err != nil {
		fmt.Printf("Verification failed: %v\n", err)
		return nil, err
	} else {
		fmt.Println("Verification succeeded")
	}

	return &DataProofResponse{
		Point:         kzgProof.Point,
		Claim:         claimArray,
		KZGCommitment: kzgCommitment,
		KZGProof:      kzgProof.Proof,
		DataProof:     dataProof,
		VersionedHash: versionedHash,
	}, nil
}

// GenerateDataProofFromSidecar loop through all blobs in the sidecar and generate data proof
// call GenerateDataProofFromBlob for each blob
// collect data proof in a slice
func GenerateDataProofFromSidecar(sideCar *gethTypes.BlobTxSidecar) ([]*DataProofResponse, error) {
	var dataProofResponses []*DataProofResponse

	for _, blob := range sideCar.Blobs {

		dataProof, dataProofGenerationError := GenerateDataProofFromBlob(&blob)

		if dataProofGenerationError != nil {
			return nil, dataProofGenerationError
		}

		dataProofResponses = append(dataProofResponses, dataProof)
	}

	// return the slice of proofs
	return dataProofResponses, nil
}

func blobDataProofFromValues(point kzg4844.Point,
	claim kzg4844.Claim,
	kzgCommitment kzg4844.Commitment,
	kzgProof kzg4844.Proof) []byte {
	result := make([]byte, 32+32+48+48)

	copy(result[0:32], point[:])
	copy(result[32:64], claim[:])
	copy(result[64:112], kzgCommitment[:])
	copy(result[112:160], kzgProof[:])

	return result
}
