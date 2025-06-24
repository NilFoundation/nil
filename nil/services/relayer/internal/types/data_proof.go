package types

import "github.com/ethereum/go-ethereum/crypto/kzg4844"

const (
	kzgPointSize      = len(kzg4844.Point{})
	kzgClaimSize      = len(kzg4844.Claim{})
	kzgCommitmentSize = len(kzg4844.Commitment{})
	kzgProofSize      = len(kzg4844.Proof{})
	kzgDataProofSize  = kzgPointSize + kzgClaimSize + kzgCommitmentSize + kzgProofSize
)

type DataProof [kzgDataProofSize]byte

func NewDataProof(
	evalPoint kzg4844.Point,
	evalClaim kzg4844.Claim,
	blobCommitment kzg4844.Commitment,
	validityProof kzg4844.Proof,
) DataProof {
	var dataProof DataProof

	copyToDataProof(&dataProof, evalPoint[:], 0)

	const claimOffset = kzgPointSize
	copyToDataProof(&dataProof, evalClaim[:], claimOffset)

	const commitmentOffset = claimOffset + kzgClaimSize
	copyToDataProof(&dataProof, blobCommitment[:], commitmentOffset)

	const proofOffset = commitmentOffset + kzgCommitmentSize
	copyToDataProof(&dataProof, validityProof[:], proofOffset)

	return dataProof
}

func copyToDataProof(proof *DataProof, data []byte, offset int) {
	targetSlice := (*proof)[offset : offset+len(data)]
	copy(targetSlice, data)
}

type DataProofs []DataProof
