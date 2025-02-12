package side_car

import (
	"errors"
	"fmt"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

func MakeSidecar(blobs ...*kzg4844.Blob) (*gethTypes.BlobTxSidecar, error) {
	if len(blobs) == 0 {
		return nil, errors.New("at least one blob must be provided")
	}

	var blobList []kzg4844.Blob
	var commitments []kzg4844.Commitment
	var proofs []kzg4844.Proof

	for _, blob := range blobs {
		if blob == nil {
			return nil, errors.New("blob cannot be nil")
		}
		blobList = append(blobList, *blob)
	}

	for i := range blobList {
		c, err := kzg4844.BlobToCommitment(&blobList[i])
		if err != nil {
			return nil, fmt.Errorf("failed to get blob commitment, err: %w", err)
		}

		p, err := kzg4844.ComputeBlobProof(&blobList[i], c)
		if err != nil {
			return nil, fmt.Errorf("failed to compute blob proof, err: %w", err)
		}

		commitments = append(commitments, c)
		proofs = append(proofs, p)
	}

	return &gethTypes.BlobTxSidecar{
		Blobs:       blobList,
		Commitments: commitments,
		Proofs:      proofs,
	}, nil
}
