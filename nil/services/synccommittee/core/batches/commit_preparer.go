package batches

import (
	"bytes"
	"errors"
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

type Commitment struct {
	Sidecar    *ethtypes.BlobTxSidecar
	DataProofs types.DataProofs
}

func NewCommitment(sidecar *ethtypes.BlobTxSidecar, dataProofs types.DataProofs) *Commitment {
	return &Commitment{
		Sidecar:    sidecar,
		DataProofs: dataProofs,
	}
}

type CommitPreparerConfig struct {
	MaxBlobsInTx uint
}

func DefaultCommitConfig() CommitPreparerConfig {
	return CommitPreparerConfig{
		MaxBlobsInTx: 6,
	}
}

type commitPreparer struct {
	encoder encode.BatchEncoder
	builder blob.Builder
	config  CommitPreparerConfig
	logger  logging.Logger
}

func NewCommitPreparer(
	encoder encode.BatchEncoder,
	builder blob.Builder,
	config CommitPreparerConfig,
	logger logging.Logger,
) *commitPreparer {
	return &commitPreparer{
		encoder: encoder,
		builder: builder,
		config:  config,
		logger:  logger,
	}
}

func (p *commitPreparer) PrepareBatchCommitment(batch *types.BlockBatch) (*Commitment, error) {
	if batch.IsEmpty() {
		return nil, errors.New("cannot prepare commitment for empty batch")
	}

	p.logger.Info().Stringer(logging.FieldBatchId, batch.Id).Msg("Preparing batch commitment")
	startTime := time.Now()

	var encodedBatch bytes.Buffer
	if err := p.encoder.Encode(types.NewPrunedBatch(batch), &encodedBatch); err != nil {
		return nil, err
	}

	p.logger.Debug().
		Int("compressedBatchLen", encodedBatch.Len()).
		Stringer(logging.FieldBatchId, batch.Id).
		Msg("Batch is encoded, packing it into blobs")

	blobs, err := p.builder.MakeBlobs(&encodedBatch, p.config.MaxBlobsInTx)
	if err != nil {
		return nil, err
	}

	sidecar, err := p.computeSidecar(blobs)
	if err != nil {
		return nil, err
	}

	dataProofs, err := p.computeDataProofs(sidecar)
	if err != nil {
		return nil, err
	}

	p.logger.Info().
		Stringer(logging.FieldBatchId, batch.Id).
		Dur(logging.FieldDuration, time.Since(startTime)).
		Msg("Batch commitment is prepared")

	return NewCommitment(sidecar, dataProofs), nil
}

func (p *commitPreparer) computeSidecar(blobs []kzg4844.Blob) (*ethtypes.BlobTxSidecar, error) {
	commitments := make([]kzg4844.Commitment, 0, len(blobs))
	proofs := make([]kzg4844.Proof, 0, len(blobs))

	p.logger.Debug().
		Int("blobCount", len(blobs)).
		Msg("Computing blob proof")

	startTime := time.Now()
	for _, kzgBlob := range blobs {
		commitment, err := kzg4844.BlobToCommitment(&kzgBlob)
		if err != nil {
			return nil, fmt.Errorf("computing commitment: %w", err)
		}

		proof, err := kzg4844.ComputeBlobProof(&kzgBlob, commitment)
		if err != nil {
			return nil, fmt.Errorf("computing proof: %w", err)
		}

		commitments = append(commitments, commitment)
		proofs = append(proofs, proof)
	}

	p.logger.Debug().
		Dur(logging.FieldDuration, time.Since(startTime)).
		Int("blobCount", len(blobs)).
		Msg("Blob proof computed")

	return &ethtypes.BlobTxSidecar{
		Blobs:       blobs,
		Commitments: commitments,
		Proofs:      proofs,
	}, nil
}

func (p *commitPreparer) computeDataProofs(sidecar *ethtypes.BlobTxSidecar) (types.DataProofs, error) {
	p.logger.Debug().
		Int("blobCount", len(sidecar.Blobs)).
		Msgf("Computing data proof")

	startTime := time.Now()

	blobHashes := sidecar.BlobHashes()
	dataProofs := make(types.DataProofs, len(blobHashes))

	for i, blobHash := range blobHashes {
		point := p.generatePointFromVersionedHash(blobHash)
		proof, claim, err := kzg4844.ComputeProof(&sidecar.Blobs[i], point)
		if err != nil {
			return nil, fmt.Errorf("failed to generate KZG proof from the blob and point at index %d: %w", i, err)
		}
		dataProofs[i] = types.NewDataProof(point, claim, sidecar.Commitments[i], proof)
	}

	p.logger.Debug().
		Dur(logging.FieldDuration, time.Since(startTime)).
		Int("blobCount", len(sidecar.Blobs)).
		Msg("Data proofs computed")

	return dataProofs, nil
}

var blsModulo = createBlsModulo()

func createBlsModulo() *big.Int {
	blsMod, set := new(big.Int).SetString(
		"52435875175126190479447740508185965837690552500527637822603658699938581184513", 10,
	)
	check.PanicIff(!set, "failed to set blsModulo")
	return blsMod
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
