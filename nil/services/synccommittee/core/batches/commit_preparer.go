package batches

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/blob"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type Commitment struct{}

type commitPreparer struct {
	encoder encode.BatchEncoder
	builder blob.Builder
	logger  logging.Logger
}

func (p *commitPreparer) PrepareForBatchCommit(
	ctx context.Context, batch *types.BlockBatch,
) (*ethtypes.BlobTxSidecar, types.DataProofs, error) {
	var binTransactions bytes.Buffer
	if err := p.encoder.Encode(types.NewPrunedBatch(batch), &binTransactions); err != nil {
		return nil, nil, err
	}
	p.logger.Debug().Int("compressed_batch_len", binTransactions.Len()).Msg("encoded transaction")
}

func (p *commitPreparer) prepareForBatchCommit(
	ctx context.Context, batch *types.BlockBatch,
) (*ethtypes.BlobTxSidecar, types.DataProofs, error) {
	var binTransactions bytes.Buffer
	if err := p.encoder.Encode(types.NewPrunedBatch(batch), &binTransactions); err != nil {
		return nil, nil, err
	}
	p.logger.Debug().Int("compressed_batch_len", binTransactions.Len()).Msg("encoded transaction")

	blobs, err := p.blobBuilder.MakeBlobs(&binTransactions, p.config.MaxBlobsInTx)
	if err != nil {
		return nil, nil, err
	}
}

func (p *commitPreparer) computeSidecar(blobs []kzg4844.Blob) (*ethtypes.BlobTxSidecar, error) {
	commitments := make([]kzg4844.Commitment, 0, len(blobs))
	proofs := make([]kzg4844.Proof, 0, len(blobs))

	startTime := time.Now()
	for _, blob := range blobs {
		commitment, err := kzg4844.BlobToCommitment(&blob)
		if err != nil {
			return nil, fmt.Errorf("computing commitment: %w", err)
		}

		proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
		if err != nil {
			return nil, fmt.Errorf("computing proof: %w", err)
		}

		commitments = append(commitments, commitment)
		proofs = append(proofs, proof)
	}
	p.logger.Info().Dur("elapsedTime", time.Since(startTime)).Int("blobsLen", len(blobs)).Msg("blob proof computed")

	return &ethtypes.BlobTxSidecar{
		Blobs:       blobs,
		Commitments: commitments,
		Proofs:      proofs,
	}, nil
}

func (p *commitPreparer) computeDataProofs(
	ctx context.Context, sidecar *ethtypes.BlobTxSidecar,
) (types.DataProofs, error) {
	blobHashes := sidecar.BlobHashes()
	dataProofs := make(types.DataProofs, len(blobHashes))
	startTime := time.Now()
	for i, blobHash := range blobHashes {
		point := p.generatePointFromVersionedHash(blobHash)
		proof, claim, err := kzg4844.ComputeProof(&sidecar.Blobs[i], point)
		if err != nil {
			return nil, fmt.Errorf("failed to generate KZG proof from the blob and point: %w", err)
		}
		dataProofs[i] = p.encodeDataProof(point, claim, sidecar.Commitments[i], proof)
	}
	p.logger.Info().
		Dur("elapsedTime", time.Since(startTime)).Int("blobsLen", len(blobHashes)).Msg("data proofs computed")

	// to make sure proofs are correct. Not necessary, if other code is not buggy
	if err := p.verifyDataProofs(ctx, sidecar.BlobHashes(), dataProofs); err != nil {
		return nil, fmt.Errorf("generated data proofs verification failed: %w", err)
	}

	return dataProofs, nil
}

var blsModulo = createBlsModulo()

func createBlsModulo() *big.Int {
	var set bool
	blsModulo, set = new(big.Int).SetString(
		"52435875175126190479447740508185965837690552500527637822603658699938581184513", 10,
	)
	check.PanicIff(!set, "failed to set blsModulo")
	return blsModulo
}

func (commitPreparer) generatePointFromVersionedHash(versionedHash ethcommon.Hash) kzg4844.Point {
	pointHash := common.Keccak256Hash(versionedHash[:])

	pointBigInt := new(big.Int).SetBytes(pointHash.Bytes())
	pointBytes := new(big.Int).Mod(pointBigInt, blsModulo).Bytes()
	start := 32 - len(pointBytes)
	var point kzg4844.Point
	copy(point[start:], pointBytes)

	return point
}

func (commitPreparer) encodeDataProof(
	point kzg4844.Point,
	claim kzg4844.Claim,
	commitment kzg4844.Commitment,
	proof kzg4844.Proof,
) []byte {
	result := make([]byte, 32+32+48+48)

	copy(result[0:32], point[:])
	copy(result[32:64], claim[:])
	copy(result[64:112], commitment[:])
	copy(result[112:160], proof[:])

	return result
}
