package proof

import (
	"fmt"
	blobutil "l2-blob-proof-playground/blob-util"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type KZGProofResponse struct {
	Point kzg4844.Point
	Claim kzg4844.Claim
	Proof kzg4844.Proof
}

func GenerateKZGProofFromBlob(blob kzg4844.Blob) (*KZGProofResponse, error) {
	// Derive the commitment from the blob
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		return nil, fmt.Errorf("failed to derive commitment from blob: %w", err)
	}

	// Derive the versionedHash from the commitment
	versionedHash := blobutil.GenerateVersionedHashFromCommitment(commitment)

	// Derive the point from the versionedHash
	point, err := GeneratePointFromVersionedHash(versionedHash.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to generate point from versionedHash: %w", err)
	}

	// Compute the proof and claim
	proof, claim, err := kzg4844.ComputeProof(&blob, point)
	if err != nil {
		return nil, fmt.Errorf("failed to generate KZG proof from the blob and point: %w", err)
	}

	return &KZGProofResponse{
		Point: point,
		Claim: claim,
		Proof: proof,
	}, nil
}

func GeneratePointFromVersionedHash(versionedHash []byte) (kzg4844.Point, error) {
	blsModulo, _ := new(big.Int).SetString("52435875175126190479447740508185965837690552500527637822603658699938581184513", 10)
	pointHash := crypto.Keccak256Hash(versionedHash)
	//fmt.Printf("pointHash: %s\n", hex.EncodeToString(pointHash.Bytes()))

	pointBigInt := new(big.Int).SetBytes(pointHash.Bytes())
	pointBytes := new(big.Int).Mod(pointBigInt, blsModulo).Bytes()
	start := 32 - len(pointBytes)
	var point kzg4844.Point
	copy(point[start:], pointBytes)
	//fmt.Printf("point: %s\n", hex.EncodeToString(point[:]))

	return point, nil
}

func GeneratePointForABlob(blob *kzg4844.Blob) (kzg4844.Point, error) {

	commitment, err := kzg4844.BlobToCommitment(blob)
	if err != nil {
		return kzg4844.Point{}, fmt.Errorf("failed to derive commitment from blob: %w", err)
	}

	versionedHash := blobutil.GenerateVersionedHashFromCommitment(commitment)

	point, err := GeneratePointFromVersionedHash(versionedHash.Bytes())
	if err != nil {
		return kzg4844.Point{}, fmt.Errorf("failed to generate point from versionedHash: %w", err)
	}

	return point, nil
}
