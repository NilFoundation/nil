package blobutil

import (
	"crypto/rand"
	"crypto/sha256"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

func GenerateRandomBlob() (*kzg4844.Blob, error) {
	var blob kzg4844.Blob
	for i := 0; i < len(blob); i += gokzg4844.SerializedScalarSize {
		fieldElementBytes := GenerateRandomFieldElement()
		copy(blob[i:i+gokzg4844.SerializedScalarSize], fieldElementBytes[:])
	}
	return &blob, nil
}

func GenerateRandomFieldElement() [32]byte {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		panic("failed to get random field element")
	}
	var r fr.Element
	r.SetBytes(bytes)

	return gokzg4844.SerializeScalar(r)
}

func GenerateVersionedHashFromCommitment(commitment kzg4844.Commitment) common.Hash {
	hasher := sha256.New()
	versionedHash := kzg4844.CalcBlobHashV1(hasher, &commitment)
	return versionedHash
}

func GetVersionedHashFromBlob(blob *kzg4844.Blob) common.Hash {
	commitment, err := kzg4844.BlobToCommitment(blob)
	if err != nil {
		panic(err)
	}
	return GenerateVersionedHashFromCommitment(commitment)
}
